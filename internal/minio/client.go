package minio

import (
	"context"
	"errors"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	corev1 "k8s.io/api/core/v1"
)

const defaultLocation = "us-east-1"

type Client struct {
	c *minio.Client
}

func NewClient(endpoint, user, password string) (*Client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(user, password, ""),
	})
	if err != nil {
		return nil, err
	}
	return &Client{c: minioClient}, nil
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

func (c *Client) NewBucket(ctx context.Context, name string) error {
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
