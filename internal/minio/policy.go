package minio

import (
	"encoding/json"

	"github.com/minio/pkg/v3/policy"
)

func DefaultPolicy(bucketName string) *policy.Policy {
	return &policy.Policy{
		Version: policy.DefaultVersion,
		Statements: []policy.Statement{
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.GetBucketLocationAction:          {},
					policy.ListBucketAction:                 {},
					policy.ListBucketMultipartUploadsAction: {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName): {},
				},
			},
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.ListMultipartUploadPartsAction: {},
					policy.PutObjectAction:                {},
					policy.AbortMultipartUploadAction:     {},
					policy.DeleteObjectAction:             {},
					policy.GetObjectAction:                {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName + "/*"): {},
				},
			},
		},
	}
}

func DefaultPolicyJSON(bucketName string) ([]byte, error) {
	return json.Marshal(DefaultPolicy(bucketName))
}

func ParsePolicy() *policy.Policy {
	return nil
}
