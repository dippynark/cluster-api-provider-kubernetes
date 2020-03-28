# Image URL to use all building/pushing image targets
IMG ?= dippynark/cluster-api-kubernetes-controller:dev
# We set maxDescLen=0 to drop descriptions for fields in CRD OpenAPI schema, otherwise annotations
# become too large when applying the kubernetesmachine and kubernetesmachinetemplate CRDs
# https://github.com/coreos/prometheus-operator/issues/535
# https://github.com/kubernetes-sigs/controller-tools/blob/0dd9d80ad4b98900d6066141dd4233354b25e3f3/pkg/crd/gen.go#L56-L61
CRD_OPTIONS ?= "crd:crdVersions=v1,maxDescLen=0"

CONTROLLER_TOOLS_VERSION = v0.2.8

# Make sure to update e2e/e2e.conf if either of these variables are changed
CAPI_VERSION = v0.3.3
CERT_MANAGER_VERSION = v0.11.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config | kubectl apply -f -
	# TODO: use aggregation label when available
	kubectl apply -f release/kubeadm-control-plane-rbac.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	hack/remove-containers-requirement.sh config/crd/bases

modules:
	go mod tidy

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run tests
test: generate fmt vet manifests
	go test $(shell go list ./... | grep -v /e2e) -coverprofile cover.out

SKIP_RESOURCE_CLEANUP ?= false
e2e: docker-build
	cd config/manager && kustomize edit set image controller=${IMG}
	go test ./e2e -v -ginkgo.v -ginkgo.trace -count=1 -timeout=20m -tags=e2e -skip-resource-cleanup=$(SKIP_RESOURCE_CLEANUP)

e2e_pull:
	docker pull gcr.io/k8s-staging-cluster-api/cluster-api-controller:$(CAPI_VERSION)
	docker pull gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controller:$(CAPI_VERSION)
	docker pull gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller:$(CAPI_VERSION)
	docker pull quay.io/jetstack/cert-manager-webhook:$(CERT_MANAGER_VERSION)
	docker pull quay.io/jetstack/cert-manager-controller:$(CERT_MANAGER_VERSION)
	docker pull quay.io/jetstack/cert-manager-cainjector:$(CERT_MANAGER_VERSION)
	docker pull kindest/node:v1.17.0

release: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/kubeadm-control-plane-rbac > release/kubeadm-control-plane-rbac.yaml
	kustomize build config > release/infrastructure-components.yaml

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
