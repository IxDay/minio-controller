package minio

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/IxDay/api/v1alpha1"
	"github.com/minio/pkg/v3/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bucketName = "test"
var policyUpload = mustMarshal(PolicyUpload(bucketName))
var policyDownload = mustMarshal(PolicyDownload(bucketName))
var policyPublic = mustMarshal(PolicyPublic(bucketName))
var policyPrivate = []byte("")

var decidePolicyEntries = []struct {
	current       []byte
	wanted        BucketPolicy
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
		gotBytes, gotErr := decidePolicy(bucketName, string(entry.current), entry.wanted)
		assert.ErrorIs(t, entry.expectedErr, gotErr)
		if bytes.Equal(gotBytes, entry.expectedBytes) {
			continue
		}
		require.NotNil(t, gotBytes)
		got, expected := &bucketPolicy{}, &policy.BucketPolicy{}
		require.NoError(t, got.UnmarshalJSON(gotBytes))
		require.NoError(t, expected.UnmarshalJSON(entry.expectedBytes))

		assert.True(t, got.Equals(*expected), "policy not equal")

	}
}
