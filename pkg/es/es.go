package es

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"com.ramos/es-provisioner/pkg/model"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var ctx = context.Background()

type EsClient struct {
	client *elasticsearch.Client
	url    string
	env    string
}

type EsService interface {
	InitializeIndex(ops *EsSetupOptions) (*EsResult, error)
	RemoveIndex(ops *EsRemoveOptions) error
}

type EsResult struct {
	UserName string
	Password string
	Role     string
	Index    string
	Alias    string
}

type EsOptions struct {
	Connection string
	Retries    int
	Username   string
	Password   string
}

type EsSetupOptions struct {
	Shards          int
	RefreshInterval string
	Replicas        int
	Namespace       string
	IndexName       string
	App             string
	Spec            string
	Analyzers       string
	Properties      string
	Source          bool
}

type EsRemoveOptions struct {
	Index string
	Alias string
	Role  string
	User  string
}

func (c *EsClient) InitializeIndex(ops *EsSetupOptions) (*EsResult, error) {

	log.Info("Creating Index...")

	var name string

	if ops.IndexName == "" {
		if ops.Namespace == "" {
			name = "es-provisioner-" + uuid.New().String()[:8]
		} else {
			name = "es-provisioner-" + ops.App + "-" + ops.Namespace
		}

	} else {
		name = ops.IndexName
	}

	indexName, aliasName, e := c.createIndex(name, ops.Spec, ops.Shards,
		ops.Replicas, ops.RefreshInterval, ops.Analyzers, ops.Source, ops.Properties)
	if e != nil {
		log.Errorf("Error creating Index %s. Error: %s", indexName, e.Error())
		return nil, e
	}

	log.Info("Creating Role...")
	roleName, e := c.createRole(indexName, aliasName, ops.App, ops.Namespace)
	if e != nil {
		log.Errorf("Error creating Role. ERROR: %s", e.Error())
		return nil, e
	}

	log.Info("Creating User...")
	userName, pw, e := c.createUser(indexName, roleName)
	if e != nil {
		log.Errorf("Error creating User ERROR: %s", e.Error())
		return nil, e
	}

	log.Info("Testing credentials")
	esOps := EsOptions{
		Connection: c.url,
		Retries:    1,
		Username:   userName,
		Password:   pw,
	}
	userClient, e := connectEsWithRetry(&esOps, 5*time.Second)
	if e != nil {
		log.Errorf("Error testing credentials ERROR: %s", e.Error())
		return nil, e
	}

	e = testIndex(userClient, indexName)
	if e != nil {
		log.Errorf("Error testing credentials ERROR: %s", e.Error())
		return nil, e
	}

	return &EsResult{
		UserName: userName,
		Password: pw,
		Role:     roleName,
		Index:    indexName,
		Alias:    aliasName,
	}, nil
}

func (c *EsClient) RemoveIndex(ops *EsRemoveOptions) error {
	e := c.deleteAlias(ops.Index, ops.Alias)
	if e != nil {
		return e
	}
	e = c.deleteIndex(ops.Index)
	if e != nil {
		return e
	}
	e = c.deleteUser(ops.User)
	if e != nil {
		return e
	}
	e = c.deleteRole(ops.Role)
	if e != nil {
		return e
	}
	return nil
}

// NewEsService Creates new Service
func NewEsService(ops *EsOptions) (EsService, error) {

	log.Infof("NewEsService, connection %s", ops.Connection)

	client, e := connectEsWithRetry(ops, 5*time.Second)
	if e != nil {
		return nil, e
	}

	_, e = client.Ping()
	if e != nil {
		return nil, e
	}

	c := &EsClient{
		client: client,
		url:    ops.Connection,
	}

	return c, nil

}

func (c *EsClient) deleteAlias(index string, alias string) error {

	log.Infof("Delete Alias: %s", alias)
	res, err := c.client.Indices.DeleteAlias([]string{index}, []string{alias})

	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("Cannot delete Alias: %s", err)
	}
	if res.IsError() {
		return fmt.Errorf("Cannot delete Alias: %s", res.String())

	}

	return nil
}

func (c *EsClient) deleteIndex(index string) error {

	log.Infof("Delete Index: %s", index)
	res, err := c.client.Indices.Delete([]string{index})

	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("Cannot delete index: %s", err)
	}
	if res.IsError() {
		return fmt.Errorf("Cannot delete index: %s", res.String())

	}

	return nil
}

func (c *EsClient) deleteUser(user string) error {

	log.Infof("Delete User: %s", user)
	res, err := c.client.Security.DeleteUser(user)

	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("Cannot delete User: %s", err)
	}
	if res.IsError() {
		return fmt.Errorf("Cannot delete User: %s", res.String())

	}

	return nil
}

func (c *EsClient) deleteRole(role string) error {

	log.Infof("Delete Role: %s", role)
	res, err := c.client.Security.DeleteRole(role)

	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("Cannot delete Role: %s", err)
	}
	if res.IsError() {
		return fmt.Errorf("Cannot delete Role: %s", res.String())

	}

	return nil
}

func connectEsWithRetry(ops *EsOptions, sleep time.Duration) (e *elasticsearch.Client, err error) {
	for i := 0; i < ops.Retries; i++ {
		if i > 0 {
			log.Warnf("retrying after error: %v, Attempt: %v", err, i)
			time.Sleep(sleep)
			sleep *= 2
		}
		c, err := connectEs(ops)
		if err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("after %d attempts, last error: %s", ops.Retries, err)
}

func connectEs(ops *EsOptions) (*elasticsearch.Client, error) {

	cfg := elasticsearch.Config{
		Addresses:           []string{ops.Connection},
		MaxRetries:          ops.Retries,
		CompressRequestBody: true,
		RetryOnStatus:       []int{429, 502, 503, 504, 500},
		RetryBackoff: func(i int) time.Duration {
			d := time.Duration(math.Exp2(float64(i))) * time.Second
			log.Warnf("Attempt: %d | Sleeping for %s...\n", i, d)
			return d
		},
	}

	log.Infof("ES Username: %s", ops.Username)
	if ops.Username != "" {
		cfg.Username = ops.Username
	}
	if ops.Password != "" {
		cfg.Password = ops.Password
	}

	log.Debugf("ES Conf: %v", cfg)
	return elasticsearch.NewClient(cfg)

}

func testIndex(c *elasticsearch.Client, index string) error {

	log.Infof("Testing index Access: %s", index)
	res, err := c.Indices.Get([]string{index})

	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("Cannot test index: %s", err)
	}
	if res.IsError() {
		return fmt.Errorf("Cannot test index: %s", res.String())

	}

	return nil
}

func (c *EsClient) createRole(index string, aliasName string, app string, namespace string) (string, error) {

	roleName := app + "-" + namespace + "-role"
	body := fmt.Sprintf(model.ROLE_TEMPLATE, index, aliasName)
	log.Infof("Creating Role: %s", roleName)
	log.Infof("Sending request: %s", body)
	res, err := c.client.Security.PutRole(roleName, strings.NewReader(body))

	defer res.Body.Close()
	if err != nil {
		return "", fmt.Errorf("Cannot create role: %s", err)
	}
	if res.IsError() {
		return "", fmt.Errorf("Cannot create role: %s", res.String())

	}

	return roleName, nil
}

func (c *EsClient) createUser(index string, role string) (string, string, error) {

	userName := role + "-user"
	pw := uuid.New().String()
	log.Infof("Creating User: %s", userName)

	body := fmt.Sprintf(model.USER_TEMPLATE, pw, role, userName)
	log.Infof("Sending request: %s", body)
	res, err := c.client.Security.PutUser(userName, strings.NewReader(body))

	defer res.Body.Close()
	if err != nil {
		return "", "", fmt.Errorf("Cannot create user: %s", err)
	}
	if res.IsError() {
		return "", "", fmt.Errorf("Cannot create user: %s", res.String())

	}

	return userName, pw, nil
}

func (c *EsClient) createIndex(name string, schema string, shards int, replicas int,
	refresh string, analyzers string, source bool, props string) (string, string, error) {

	indexName := name + "-" + time.Now().Format(time.RFC3339)[:10]
	log.Infof("Creating Index: %s", indexName)

	var body string
	if schema == "" {
		if analyzers == "" {
			analyzers = "standard"
		}

		if shards == 0 {
			shards = 4
		}

		if refresh == "" {
			refresh = "30s"
		}
		body = fmt.Sprintf(model.INDEX_TEMPLATE, shards, replicas, refresh, analyzers, source, props)
	} else {
		body = fmt.Sprintf(schema, shards, replicas, refresh)
	}

	log.Debugf("Creating Index with Body: %s", body)
	res, err := c.client.Indices.Create(indexName, func(val *esapi.IndicesCreateRequest) {
		(*val).Body = strings.NewReader(body)
	})
	defer res.Body.Close()
	if err != nil {
		return "", "", fmt.Errorf("Cannot create index: %s", err)
	}
	if res.IsError() {
		if !strings.Contains(res.String(), "resource_already_exists_exception") {
			return "", "", fmt.Errorf("Cannot create index: %s", res.String())
		}

	}

	return c.addAlias(indexName, name)
}

func (c *EsClient) addAlias(indexName string, aliasName string) (string, string, error) {

	log.Infof("Creating Alias: %s", aliasName)
	res, err := c.client.Indices.PutAlias([]string{indexName}, aliasName)
	defer res.Body.Close()
	if err != nil {
		return "", "", fmt.Errorf("Cannot create alias: %s", err)
	}
	if res.IsError() {
		if !strings.Contains(res.String(), "resource_already_exists_exception") {
			return "", "", fmt.Errorf("Cannot create alias: %s", res.String())
		}

	}
	return indexName, aliasName, nil
}
