package minio

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/IxDay/api/v1alpha1"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/pkg/v3/policy"

	corev1 "k8s.io/api/core/v1"
)

const defaultLocation = "us-east-1"

type BucketClient = minio.Client
type BucketPolicy = v1alpha1.BucketPolicy

type Client interface {
	BucketCreate(ctx context.Context, name string) error
	BucketDelete(ctx context.Context, name string) error
	BucketExists(ctx context.Context, name string) (bool, error)
	BucketPolicyReconcile(ctx context.Context, name string, policy BucketPolicy) error
	PolicyReconcile(ctx context.Context, policy *Policy) error
	PolicyDelete(ctx context.Context, name string) error
}

type client struct {
	*minio.Client
	*madmin.AdminClient
}

func NewClient(endpoint, user, password string) (Client, error) {
	creds := credentials.NewStaticV4(user, password, "")
	minioClient, err := minio.New(endpoint, &minio.Options{Creds: creds})
	if err != nil {
		return nil, err
	}
	minioAdminClient, err := madmin.NewWithOptions(endpoint, &madmin.Options{Creds: creds})
	if err != nil {
		return nil, err
	}
	return &client{Client: minioClient, AdminClient: minioAdminClient}, nil
}

var ErrInvalidSecret = errors.New("invalid secret format")

func NewClientFromSecret(secret *corev1.Secret) (Client, error) {
	var (
		endpoint = string(secret.Data["endpoint"])
		user     = string(secret.Data["user"])
		password = string(secret.Data["password"])
	)
	if endpoint == "" || user == "" || password == "" {
		return nil, ErrInvalidSecret
	}
	return NewClient(endpoint, user, password)
}

func NewMinioClientFromSecret(secret *corev1.Secret) (*minio.Client, error) {
	c, err := NewClientFromSecret(secret)
	if err != nil {
		return nil, err
	}
	return c.(*client).Client, nil
}

func (c *client) BucketCreate(ctx context.Context, name string) error {
	opts := minio.MakeBucketOptions{Region: defaultLocation}

	if err := c.MakeBucket(ctx, name, opts); err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		// exists, errBucketExists := c.c.BucketExists(ctx, name)
		// if errBucketExists == nil && exists {
		// 	log.Printf("We already own %s\n", bucketName)
		// } else {
		// 	log.Fatalln(err)
		// }
		return err
	}
	return nil
}

func (c *client) BucketExists(ctx context.Context, name string) (bool, error) {
	return c.Client.BucketExists(ctx, name)
}

func (c *client) BucketDelete(ctx context.Context, name string) error {
	return c.RemoveBucket(ctx, name)
}

func (c *client) policyCreate(ctx context.Context, policy *Policy) error {
	p, err := json.Marshal(policy.Policy)
	if err != nil {
		return err
	}

	if err := c.AddCannedPolicy(ctx, policy.Name, p); err != nil {
		return err
	}
	return c.userCreate(ctx, policy)
}

func (c *client) userCreate(ctx context.Context, policy *Policy) error {
	if err := c.AddUser(ctx, policy.User.Name, policy.User.Password); err != nil {
		return err
	}
	association := madmin.PolicyAssociationReq{
		Policies: []string{policy.Name},
		User:     policy.User.Name,
	}
	if _, err := c.AttachPolicy(ctx, association); err != nil {
		return err
	}
	return nil
}

func (c *client) PolicyReconcile(ctx context.Context, policy *Policy) error {
	entities := madmin.PolicyEntitiesQuery{Policy: []string{policy.Name}}
	results, err := c.GetPolicyEntities(ctx, entities)
	if err != nil {
		return err
	}
	if len(results.PolicyMappings) == 0 {
		return c.policyCreate(ctx, policy)
	}
	if len(results.PolicyMappings[0].Users) == 0 {
		return c.userCreate(ctx, policy)
	}
	user := results.PolicyMappings[0].Users[0]
	if policy.User.Name == user {
		// update current user with password
		return c.SetUser(ctx, policy.User.Name, policy.User.Password, madmin.AccountEnabled)
	}
	// delete old user and set new one
	if err := c.RemoveUser(ctx, user); err != nil {
		return err
	}
	return c.userCreate(ctx, policy)
}

func (c *client) PolicyDelete(ctx context.Context, policy string) error {
	query := madmin.PolicyEntitiesQuery{Policy: []string{policy}}
	result, err := c.GetPolicyEntities(ctx, query)
	if err != nil {
		return err
	}
	if result.PolicyMappings == nil {
		return nil
	}
	errs := []error{}
	for _, user := range result.PolicyMappings[0].Users {
		if err := c.RemoveUser(ctx, user); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(append(errs, c.RemoveCannedPolicy(ctx, policy))...)
}

func (c *client) BucketPolicyReconcile(ctx context.Context, name string, policy BucketPolicy) error {
	current, err := c.GetBucketPolicy(ctx, name)
	if err != nil {
		return err
	}
	expected, err := decidePolicy(name, current, policy)
	if err != nil {
		return err
	}
	if expected == nil {
		return nil
	}
	return c.SetBucketPolicy(ctx, name, string(expected))
}

type bucketPolicy = policy.BucketPolicy

func decidePolicy(bucket, currentJSON string, policy BucketPolicy) ([]byte, error) {

	if policy == v1alpha1.PolicyPrivate && currentJSON == "" {
		return nil, nil
	}
	if policy == v1alpha1.PolicyPrivate && currentJSON != "" {
		return []byte{}, nil
	}

	current, expected := &bucketPolicy{}, bucketPolicy{}
	switch policy {
	case v1alpha1.PolicyPublic:
		expected = *PolicyPublic(bucket)
	case v1alpha1.PolicyDownload:
		expected = *PolicyDownload(bucket)
	case v1alpha1.PolicyUpload:
		expected = *PolicyUpload(bucket)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return nil, err
	}
	if currentJSON == "" {
		return expectedJSON, nil
	}
	if err := current.UnmarshalJSON([]byte(currentJSON)); err != nil {
		return nil, err
	}
	if current.Equals(expected) {
		return nil, nil
	}
	return expectedJSON, nil
}

type stub struct{}

func (s stub) BucketCreate(context.Context, string) error                        { return nil }
func (s stub) BucketExists(context.Context, string) (bool, error)                { return true, nil }
func (s stub) BucketDelete(context.Context, string) error                        { return nil }
func (s stub) PolicyReconcile(context.Context, *Policy) error                    { return nil }
func (s stub) PolicyDelete(context.Context, string) error                        { return nil }
func (s stub) BucketPolicyReconcile(context.Context, string, BucketPolicy) error { return nil }

func NewStub() Client { return stub{} }
