KUBEBUILDER_ASSETS="$(setup-envtest use "$K8S_VERSION" --bin-dir bin -p path)"

go test $(go list ./... | grep -v /e2e) -coverprofile cover.out
