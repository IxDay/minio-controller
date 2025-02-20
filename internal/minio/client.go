package minio

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	corev1 "k8s.io/api/core/v1"
)

const defaultLocation = "us-east-1"

type BucketClient = minio.Client

type Client interface {
	BucketCreate(ctx context.Context, name string) error
	BucketDelete(ctx context.Context, name string) error
	BucketExists(ctx context.Context, name string) (bool, error)
	PolicyCreate(ctx context.Context, policy *Policy) error
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

func (c *client) PolicyCreate(ctx context.Context, policy *Policy) error {
	if err := c.AddUser(ctx, policy.User.Name, policy.User.Password); err != nil {
		return err
	}
	p, err := json.Marshal(policy.Policy)
	if err != nil {
		return err
	}

	if err := c.AddCannedPolicy(ctx, policy.Name, p); err != nil {
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

type stub struct{}

func (s stub) BucketCreate(context.Context, string) error         { return nil }
func (s stub) BucketExists(context.Context, string) (bool, error) { return true, nil }
func (s stub) BucketDelete(context.Context, string) error         { return nil }
func (s stub) PolicyCreate(context.Context, *Policy) error        { return nil }
func (s stub) PolicyDelete(context.Context, string) error         { return nil }

func NewStub() Client { return stub{} }
