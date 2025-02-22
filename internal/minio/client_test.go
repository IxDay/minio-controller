package minio

import (
	"encoding/json"
	"testing"

	"github.com/IxDay/api/v1alpha1"
	"github.com/minio/pkg/v3/policy"
	"github.com/stretchr/testify/assert"
)

var bucketName = "test"
var policyUpload = mustMarshal(PolicyUpload(bucketName))
var policyDownload = mustMarshal(PolicyDownload(bucketName))
var policyPublic = mustMarshal(PolicyPublic(bucketName))
var policyPrivate = []byte("")

var decidePolicyEntries = []struct {
	current       []byte
	policy        BucketPolicy
	expectedBytes []byte
	expectedErr   error
}{
	{policyPrivate, v1alpha1.PolicyPrivate, nil, nil},
	{policyUpload, v1alpha1.PolicyUpload, nil, nil},
	{policyPrivate, v1alpha1.PolicyDownload, policyDownload, nil},
	{policyPublic, v1alpha1.PolicyPrivate, policyPrivate, nil},
}

func mustMarshal(policy *policy.BucketPolicy) []byte {
	bytes, err := json.Marshal(policy)
	if err != nil {
		panic(err)
	}
	return bytes
}

func Test_decidePolicy(t *testing.T) {
	for _, entry := range decidePolicyEntries {
		gotBytes, gotErr := decidePolicy(bucketName, string(entry.current), entry.policy)

		assert.ErrorIsf(t, entry.expectedErr, gotErr,
			"errors do not match - expected: %q, got: %q", entry.expectedErr, gotErr)

		assert.Equal(t, entry.expectedBytes, gotBytes,
			"bytes output do not match - expected: %q, got: %q", entry.expectedBytes, gotBytes)

	}
}
