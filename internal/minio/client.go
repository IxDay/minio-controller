package minio

import (
	"context"
	"errors"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	corev1 "k8s.io/api/core/v1"
)

const defaultLocation = "us-east-1"

type Client interface {
	UserDelete(ctx context.Context, bucket string) error
	UserCreate(ctx context.Context, user, password, bucket string) error
	BucketCreate(ctx context.Context, name string) error
	BucketDelete(ctx context.Context, name string) error
	BucketExists(ctx context.Context, name string) (bool, error)
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

func (c *client) UserCreate(ctx context.Context, user, password, bucket string) error {
	if err := c.AddUser(ctx, user, password); err != nil {
		return err
	}
	policy, err := DefaultPolicyJSON(bucket)
	if err != nil {
		return err
	}
	if err := c.AddCannedPolicy(ctx, bucket, policy); err != nil {
		return err
	}
	association := madmin.PolicyAssociationReq{
		Policies: []string{bucket},
		User:     user,
	}
	if _, err := c.AttachPolicy(ctx, association); err != nil {
		return err
	}
	return nil
}

func (c *client) UserDelete(ctx context.Context, bucket string) error {
	query := madmin.PolicyEntitiesQuery{Policy: []string{bucket}}
	result, err := c.GetPolicyEntities(ctx, query)
	if err != nil {
		return err
	}
	errs := []error{}
	for _, user := range result.PolicyMappings[0].Users {
		if err := c.RemoveUser(ctx, user); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(append(errs, c.RemoveCannedPolicy(ctx, bucket))...)
}

type stub struct{}

func (s stub) BucketCreate(context.Context, string) error               { return nil }
func (s stub) BucketExists(context.Context, string) (bool, error)       { return true, nil }
func (s stub) BucketDelete(context.Context, string) error               { return nil }
func (s stub) UserCreate(context.Context, string, string, string) error { return nil }
func (s stub) UserDelete(context.Context, string) error                 { return nil }

func NewStub() Client { return stub{} }
