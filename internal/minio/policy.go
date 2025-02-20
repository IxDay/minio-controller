package minio

import (
	"errors"

	"github.com/IxDay/api/v1alpha1"
	"github.com/minio/pkg/v3/policy"
)

var (
	ErrInvalidAction   = errors.New("invalid action")
	ErrInvalidSubPath  = errors.New("invalid subPath")
	ErrInvalidUser     = errors.New("invalid user")
	ErrInvalidPassword = errors.New("invalid password")
	empty              = struct{}{}
)

type Policy struct {
	User struct {
		Name, Password string
	}
	Name, Bucket string
	*policy.Policy
}

func (p *Policy) SetUser(user, password []byte) error {
	if p.User.Password = string(password); password == nil {
		return ErrInvalidPassword
	}
	if p.User.Name = string(user); user == nil {
		return ErrInvalidUser
	}
	return nil
}

func (p *Policy) SetPolicy(statements []v1alpha1.Statement) (err error) {
	p.Policy, err = BucketPolicy(p.Bucket, statements)
	return
}

func NewDefaultPolicy(bucketName string) *Policy {
	return &Policy{
		Name: bucketName, Bucket: bucketName,
		Policy: DefaultPolicy(bucketName),
	}
}

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
