package minio

import (
	"testing"

	"github.com/minio/pkg/v3/policy"
	"github.com/stretchr/testify/assert"
)

var defaultPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetBucketLocation",
        "s3:ListBucket",
        "s3:ListBucketMultipartUploads"
      ],
      "Resource": [
        "arn:aws:s3:::foo"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListMultipartUploadParts",
        "s3:PutObject",
        "s3:AbortMultipartUpload",
        "s3:DeleteObject",
        "s3:GetObject"
      ],
      "Resource": [
        "arn:aws:s3:::foo/*"
      ]
    }
  ]
}`

func TestPublicPolicy(t *testing.T) {
	expected := &policy.Policy{}
	if err := expected.UnmarshalJSON([]byte(defaultPolicy)); err != nil {
		t.Errorf("failed to unmarshal olicy")
	}
	actual := transformPolicy(PolicyPublic("foo"))
	assert.Equal(t, expected, actual)
}
