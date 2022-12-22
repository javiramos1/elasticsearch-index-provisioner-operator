/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"strings"

	esv1 "com.ramos/es-provisioner/api/v1"
	"com.ramos/es-provisioner/pkg/es"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	indexOwnerKey = ".metadata.controller"
	finalizerName = "index.es-provisioner.com.ramos/finalizer"
	secretName    = "es-provisioner-index-secret"
	configMapKey  = "mapping.json"
)

// IndexReconciler reconciles a Index object
type IndexReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	EsService *es.EsService
	K8sClient *kubernetes.Clientset
}

//+kubebuilder:rbac:groups=es-provisioner.com.ramos,resources=indices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=es-provisioner.com.ramos,resources=indices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=es-provisioner.com.ramos,resources=indices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Index object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *IndexReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(1).Info("Got request", "req", req)
	// return ctrl.Result{}, nil
	var index esv1.Index
	if err := r.Get(ctx, req.NamespacedName, &index); err != nil {
		log.Error(err, "Index Not Found")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set finalizer for deletion hooks
	err := r.setFinalizer(ctx, &index)
	if err != nil {
		log.Error(err, "Error Setting Finalizer")
		r.updateStatus(&index, ctx, esv1.Error)
		return ctrl.Result{}, err
	}

	if index.Status.IndexStatus == "" { // if no status then we know it has just being created
		return r.provisionIndex(index, ctx, req)
	}

	return ctrl.Result{}, nil
}

func (r *IndexReconciler) provisionIndex(index esv1.Index, ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.updateStatus(&index, ctx, esv1.Creating)
	log := log.FromContext(ctx)

	ns, err := r.K8sClient.CoreV1().Namespaces().Get(ctx, req.Namespace, v1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get Namespace")
		r.updateStatus(&index, ctx, esv1.Error)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Retrieved Namespace", "namespace", &ns)

	spec, err := r.getConfigMap(ctx, &index, req.Namespace)
	if err != nil {
		r.updateStatus(&index, ctx, esv1.Error)
		return ctrl.Result{}, err
	}

	ops := es.EsSetupOptions{
		Shards:          index.Spec.NumberOfShards,
		RefreshInterval: index.Spec.RefreshInterval,
		Replicas:        index.Spec.NumberOfReplicas,
		IndexName:       index.Spec.Name,
		App:             index.Spec.Application,
		Namespace:       ns.Name,
		Spec:            spec,
		Analyzers:       index.Spec.Analyzers,
		Properties:      index.Spec.Properties,
		Source:          index.Spec.SourceEnabled,
	}

	log.V(1).Info("Provisioning Tenant in ElasticSearch", "options", &ns)

	esResult, err := (*r.EsService).InitializeIndex(&ops)
	if err != nil {
		r.updateStatus(&index, ctx, esv1.Error)
		log.Error(err, "unable setup Index")
		return ctrl.Result{}, err
	}

	r.updateStatus(&index, ctx, esv1.Created)

	log.V(1).Info("Tenant Provisioned. Creating Secret...", "result", esResult)

	err = r.createSecret(ctx, esResult, req)
	if err != nil {
		r.updateStatus(&index, ctx, esv1.Error)
		log.Error(err, "Error Creating Secret")
		return ctrl.Result{}, err
	}
	log.V(1).Info("Secret Created, Provisoned Completed.")
	r.updateStatus(&index, ctx, esv1.Ready)
	return reconcile.Result{}, nil
}

func (r *IndexReconciler) createSecret(ctx context.Context, esResult *es.EsResult, req ctrl.Request) error {

	// delete exiting
	_ = r.K8sClient.CoreV1().Secrets(req.Namespace).Delete(ctx, secretName, v1.DeleteOptions{})

	secretMeta := v1.ObjectMeta{
		Name:        secretName,
		Namespace:   req.Namespace,
		Annotations: map[string]string{"owner": "es-provisioner"},
	}

	secretData := map[string][]byte{}
	secretData["username"] = []byte(esResult.UserName)
	secretData["password"] = []byte(esResult.Password)
	secretData["index"] = []byte(esResult.Alias)
	secretData["_index"] = []byte(esResult.Index)
	secretData["role"] = []byte(esResult.Role)

	secret := coreV1.Secret{
		ObjectMeta: secretMeta,
		Data:       secretData,
	}
	_, err := r.K8sClient.CoreV1().Secrets(req.Namespace).Create(ctx, &secret, v1.CreateOptions{})

	return err
}

func (r *IndexReconciler) updateStatus(index *esv1.Index, ctx context.Context, status esv1.IndexStatusEnum) {
	log := log.FromContext(ctx)
	index.Status.IndexStatus = status
	err := r.Status().Update(ctx, index)
	if err != nil {
		log.Error(err, "Error updating status")
	}
}

func (r *IndexReconciler) getConfigMap(ctx context.Context, index *esv1.Index, namespace string) (string, error) {
	log := log.FromContext(ctx)
	var spec string
	if index.Spec.ConfigMap != "" {
		cm, err := r.K8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, index.Spec.ConfigMap, v1.GetOptions{})
		if err != nil {
			log.Error(err, "unable to get ConfigMap", "cm name", index.Spec.ConfigMap)
			return "", err
		}
		cmData := cm.Data[configMapKey]
		if cmData == "" {
			log.Error(err, "unable to get ConfigMap. Missing Key", "key", configMapKey)
			return "", err
		}
		spec = cmData
	}

	return spec, nil
}

func (r *IndexReconciler) setFinalizer(ctx context.Context, index *esv1.Index) error {
	// examine DeletionTimestamp to determine if object is under deletion
	if index.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(index, finalizerName) {
			controllerutil.AddFinalizer(index, finalizerName)
			if err := r.Update(ctx, index); err != nil {
				return err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(index, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			err := r.deleteIndex(ctx, index)

			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(index, finalizerName)
			if err := r.Update(ctx, index); err != nil {
				return err
			}

		}
	}

	return nil
}

func (r *IndexReconciler) deleteIndex(ctx context.Context, index *esv1.Index) error {
	log := log.FromContext(ctx)

	secret, err := r.K8sClient.CoreV1().Secrets(index.Namespace).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return err
	}
	indx := string(secret.Data["index"])
	log.V(1).Info("Deleting index..", "index", indx)
	ops := &es.EsRemoveOptions{
		Index: string(secret.Data["_index"]),
		Alias: indx,
		Role:  string(secret.Data["role"]),
		User:  string(secret.Data["username"]),
	}

	err = (*r.EsService).RemoveIndex(ops)
	if err != nil {
		return err
	}

	log.V(1).Info("Index removed, deleting secret", "secret", secretName)

	err = r.K8sClient.CoreV1().Secrets(index.Namespace).Delete(ctx, secretName, v1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.V(1).Info("Clean up completed")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IndexReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&esv1.Index{}).
		Complete(r)
}
