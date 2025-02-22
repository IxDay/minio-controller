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
	p.Policy, err = NewPolicy(p.Bucket, statements)
	return
}

func NewDefaultPolicy(bucketName string) *Policy {
	return &Policy{
		Name: bucketName, Bucket: bucketName,
		Policy: &policy.Policy{
			Version:    policy.DefaultVersion,
			Statements: transformStatements(PolicyPublic(bucketName).Statements),
		},
	}
}

func transformStatements(in []policy.BPStatement) (out []policy.Statement) {
	out = make([]policy.Statement, len(in))
	for i := range in {
		out[i].Effect = in[i].Effect
		out[i].Actions = in[i].Actions
		out[i].Resources = in[i].Resources
	}
	return
}

// mc anonymous set public local/<name_of_bucket>
// mc anonymous get-json local/<name_of_bucket>
func PolicyPublic(bucketName string) *policy.BucketPolicy {
	return &policy.BucketPolicy{
		Version: policy.DefaultVersion,
		Statements: []policy.BPStatement{
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
				Principal: policy.NewPrincipal("*"),
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
				Principal: policy.NewPrincipal("*"),
			},
		},
	}
}

// mc anonymous set download local/<name_of_bucket>
// mc anonymous get-json local/<name_of_bucket>
func PolicyDownload(bucketName string) *policy.BucketPolicy {
	return &policy.BucketPolicy{
		Version: policy.DefaultVersion,
		Statements: []policy.BPStatement{
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.GetBucketLocationAction: {},
					policy.ListBucketAction:        {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName): {},
				},
				Principal: policy.NewPrincipal("*"),
			},
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.GetObjectAction: {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName + "/*"): {},
				},
				Principal: policy.NewPrincipal("*"),
			},
		},
	}
}

// mc anonymous set upload local/<name_of_bucket>
// mc anonymous get-json local/<name_of_bucket>
func PolicyUpload(bucketName string) *policy.BucketPolicy {
	return &policy.BucketPolicy{
		Version: policy.DefaultVersion,
		Statements: []policy.BPStatement{
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.GetBucketLocationAction:          {},
					policy.ListBucketMultipartUploadsAction: {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName): {},
				},
				Principal: policy.NewPrincipal("*"),
			},
			{
				Effect: policy.Allow,
				Actions: policy.ActionSet{
					policy.ListMultipartUploadPartsAction: {},
					policy.PutObjectAction:                {},
					policy.AbortMultipartUploadAction:     {},
					policy.DeleteObjectAction:             {},
				},
				Resources: policy.ResourceSet{
					policy.NewResource(bucketName + "/*"): {},
				},
				Principal: policy.NewPrincipal("*"),
			},
		},
	}
}

func NewPolicy(bucketName string, statements []v1alpha1.Statement) (*policy.Policy, error) {
	p := policy.Policy{Version: policy.DefaultVersion, Statements: make([]policy.Statement, len(statements))}
	for i, statement := range statements {
		resources := make(policy.ResourceSet, len(statement.SubPaths))
		if len(resources) == 0 {
			resources = policy.ResourceSet{policy.NewResource(bucketName): {}}
		}
		for _, path := range statement.SubPaths {
			r := policy.NewResource(bucketName + "/" + path)
			if !r.IsValid() {
				return nil, ErrInvalidSubPath
			}
			resources[r] = empty
		}
		actions := make(policy.ActionSet, len(statement.SubPaths))
		for _, action := range statement.Actions {
			a := policy.Action(action)
			if !a.IsValid() {
				return nil, ErrInvalidAction
			}
			actions[a] = empty
		}

		p.Statements[i] = policy.Statement{
			Effect:    policy.Effect(statement.Effect),
			Resources: resources,
			Actions:   actions,
		}
	}
	return &p, nil
}
