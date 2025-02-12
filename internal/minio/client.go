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

type Client struct {
	c *minio.Client
	a *madmin.AdminClient
}

func NewClient(endpoint, user, password string) (*Client, error) {
	creds := credentials.NewStaticV4(user, password, "")
	minioClient, err := minio.New(endpoint, &minio.Options{Creds: creds})
	if err != nil {
		return nil, err
	}
	minioAdminClient, err := madmin.NewWithOptions(endpoint, &madmin.Options{Creds: creds})
	if err != nil {
		return nil, err
	}
	return &Client{c: minioClient, a: minioAdminClient}, nil
}

var ErrInvalidSecret = errors.New("invalid secret format")

func NewClientFromSecret(secret *corev1.Secret) (*Client, error) {
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

func (c *Client) BucketCreate(ctx context.Context, name string) error {
	opts := minio.MakeBucketOptions{Region: defaultLocation}

	if err := c.c.MakeBucket(ctx, name, opts); err != nil {
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

func (c *Client) BucketExists(ctx context.Context, name string) (bool, error) {
	return c.c.BucketExists(ctx, name)
}

func (c *Client) BucketDelete(ctx context.Context, name string) error {
	return c.c.RemoveBucket(ctx, name)
}

func (c *Client) UserCreate(ctx context.Context, user, password, bucket string) error {
	if err := c.a.AddUser(ctx, user, password); err != nil {
		return err
	}
	policy, err := DefaultPolicyJSON(bucket)
	if err != nil {
		return err
	}
	if err := c.a.AddCannedPolicy(ctx, bucket, policy); err != nil {
		return err
	}
	association := madmin.PolicyAssociationReq{
		Policies: []string{bucket},
		User:     user,
	}
	if _, err := c.a.AttachPolicy(ctx, association); err != nil {
		return err
	}
	return nil
}

func (c *Client) UserDelete(ctx context.Context, bucket string) error {
	query := madmin.PolicyEntitiesQuery{Policy: []string{bucket}}
	result, err := c.a.GetPolicyEntities(ctx, query)
	if err != nil {
		return err
	}
	errs := []error{}
	for _, user := range result.PolicyMappings[0].Users {
		if err := c.a.RemoveUser(ctx, user); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(append(errs, c.a.RemoveCannedPolicy(ctx, bucket))...)
}
