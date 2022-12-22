package model

const INDEX_TEMPLATE = `
{
	"settings": {
	  "index.number_of_shards": "%d",
	  "index.number_of_replicas": %d,
	  "index.refresh_interval": "%s",
	  "analysis": {
		"analyzer": "%s"
	  }
	},
	"mappings": {
	  "_source": {
		"enabled": %t
	  },
	  "dynamic": "strict",
	  "properties": {
		%s
	  }
	}
  }
`

const ROLE_TEMPLATE = `
{
	"indices": [
	  {
		"names": [ "%s", "%s" ],
		"privileges": ["create", "create_doc", "index", "read", "write", "view_index_metadata"]
	  }
	]
  }
`

const USER_TEMPLATE = `
{
	"password" : "%s",
	"roles" : [ "%s" ],
	"full_name" : "%s"
}
`
