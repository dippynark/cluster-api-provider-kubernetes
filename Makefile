# Image URL to use all building/pushing image targets
IMG ?= docker.io/dippynark/cluster-api-kubernetes-controller:dev
# We set maxDescLen=0 to drop descriptions for fields in CRD OpenAPI schema, otherwise annotations
# become too large when applying the kubernetesmachine and kubernetesmachinetemplate CRDs
# https://github.com/coreos/prometheus-operator/issues/535
# https://github.com/kubernetes-sigs/controller-tools/blob/0dd9d80ad4b98900d6066141dd4233354b25e3f3/pkg/crd/gen.go#L56-L61
CRD_OPTIONS ?= "crd:crdVersions=v1,maxDescLen=0"

CONTROLLER_TOOLS_VERSION = v0.5.0

CAPI_VERSION = v0.3.14
CERT_MANAGER_VERSION = v0.16.1
KUBERNETES_VERSION = v1.20.2

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

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config | kubectl apply -f -

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

release: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
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

# e2e testing

BIN_DIR := bin
GINKGO := $(BIN_DIR)/ginkgo
$(GINKGO):
	go build -tags=tools -o $(GINKGO) github.com/onsi/ginkgo/ginkgo

ARTIFACTS ?= $(CURDIR)/artifacts
E2E_CONF_FILE  ?= $(CURDIR)/e2e/config/capk.yaml
SKIP_RESOURCE_CLEANUP ?= false
USE_EXISTING_CLUSTER ?= false
.PHONY: e2e
e2e: $(GINKGO) docker-build e2e_pull e2e_template e2e_data
	cd config/manager && kustomize edit set image controller=${IMG}
	$(GINKGO) -v -trace -tags=e2e ./e2e -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE)" \
		-e2e.skip-resource-cleanup=$(SKIP_RESOURCE_CLEANUP) \
		-e2e.use-existing-cluster=$(USE_EXISTING_CLUSTER)

e2e_template:
	sed -i 's#$(shell echo $(IMG) | awk -F : '{print $$1}'):.*#$(IMG)#' $(E2E_CONF_FILE)
	sed -i 's#KUBERNETES_VERSION: .*#KUBERNETES_VERSION: "$(KUBERNETES_VERSION)"#' $(E2E_CONF_FILE)
	sed -i 's#gcr.io/k8s-staging-cluster-api/cluster-api-controller:.*#gcr.io/k8s-staging-cluster-api/cluster-api-controller:$(CAPI_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controlle:.*#gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controlle:$(CAPI_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller:.*#gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller:$(CAPI_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#quay.io/jetstack/cert-manager-webhook:.*#quay.io/jetstack/cert-manager-webhook:$(CERT_MANAGER_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#quay.io/jetstack/cert-manager-controller:.*#quay.io/jetstack/cert-manager-controller:$(CERT_MANAGER_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#quay.io/jetstack/cert-manager-cainjector:.*#quay.io/jetstack/cert-manager-cainjector:$(CERT_MANAGER_VERSION)#' $(E2E_CONF_FILE)
	sed -i 's#kindest/node:.*#kindest/node:$(KUBERNETES_VERSION)#' $(E2E_CONF_FILE)

e2e_pull:
	docker pull gcr.io/k8s-staging-cluster-api/cluster-api-controller:$(CAPI_VERSION)
	docker pull gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controller:$(CAPI_VERSION)
	docker pull gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller:$(CAPI_VERSION)
	docker pull quay.io/jetstack/cert-manager-webhook:$(CERT_MANAGER_VERSION)
	docker pull quay.io/jetstack/cert-manager-controller:$(CERT_MANAGER_VERSION)
	docker pull quay.io/jetstack/cert-manager-cainjector:$(CERT_MANAGER_VERSION)
	docker pull kindest/node:$(KUBERNETES_VERSION)

DATA_DIR = e2e/data
e2e_data:
	# Download Calico for CNI implementation
	curl https://docs.projectcalico.org/manifests/calico.yaml -o $(DATA_DIR)/cni/calico/calico.yaml
	# Copy release template
	cp release/cluster-template.yaml $(DATA_DIR)/infrastructure-kubernetes/cluster-template/cluster-template.yaml
	kustomize build $(DATA_DIR)/infrastructure-kubernetes/cluster-template > $(DATA_DIR)/infrastructure-kubernetes/cluster-template.yaml
