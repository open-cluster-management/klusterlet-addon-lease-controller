#!/bin/bash
###############################################################################
# Copyright (c) 2020 Red Hat, Inc.
###############################################################################

set -e
#set -x

CURR_FOLDER_PATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
KIND_KUBECONFIG="${CURR_FOLDER_PATH}/../test/functional/kind_kubeconfig.yaml"
export KUBECONFIG=${KIND_KUBECONFIG}
export DOCKER_IMAGE_AND_TAG=${1}


if [ -z $DOCKER_USER ]; then
   echo "DOCKER_USER is not defined!"
   exit 1
fi
if [ -z $DOCKER_PASS ]; then
   echo "DOCKER_PASS is not defined!"
   exit 1
fi

export FUNCT_TEST_TMPDIR="${CURR_FOLDER_PATH}/../test/functional/tmp"
export FUNCT_TEST_COVERAGE="${CURR_FOLDER_PATH}/../test/functional/coverage"
export HUB_KUBECONFIG_DIR="${CURR_FOLDER_PATH}/../test/functional/hub-kind-kubeconfig"

if ! which kubectl > /dev/null; then
    echo "installing kubectl"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
fi
if ! which kind > /dev/null; then
    echo "installing kind"
    curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.7.0/kind-$(uname)-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi
if ! which ginkgo > /dev/null; then
    export GO111MODULE=off
    echo "Installing ginkgo ..."
    go get github.com/onsi/ginkgo/ginkgo
    go get github.com/onsi/gomega/...
fi
if ! which gocovmerge > /dev/null; then
  echo "Installing gocovmerge..."
  go get -u github.com/wadey/gocovmerge
fi

echo "setting up test tmp folder"
[ -d "$FUNCT_TEST_TMPDIR" ] && rm -r "$FUNCT_TEST_TMPDIR"
mkdir -p "$FUNCT_TEST_TMPDIR"
# mkdir -p "$FUNCT_TEST_TMPDIR/output"
mkdir -p "$FUNCT_TEST_TMPDIR/kind-config"

echo "setting up test coverage folder"
[ -d "$FUNCT_TEST_COVERAGE" ] && rm -r "$FUNCT_TEST_COVERAGE"
mkdir -p "${FUNCT_TEST_COVERAGE}"

echo "setting up test kubeconfig folder"
[ -d "$HUB_KUBECONFIG_DIR" ] && rm -r "$HUB_KUBECONFIG_DIR"
mkdir -p "${HUB_KUBECONFIG_DIR}"

echo "generating managed-cluster kind configfile"
cat << EOF > "${FUNCT_TEST_TMPDIR}/kind-config/kind-config.yaml"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
  - hostPath: "${FUNCT_TEST_COVERAGE}"
    containerPath: /tmp/coverage
EOF

echo "generating hub cluster kind configfile"
cat << EOF > "${FUNCT_TEST_TMPDIR}/kind-config/hub-kind-config.yaml"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerPort: 6443
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration #for worker use JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        system-reserved: memory=2Gi
EOF


echo "creating hub cluster"
kind create cluster --name functional-test-hub --config "${FUNCT_TEST_TMPDIR}/kind-config/hub-kind-config.yaml"

# setup kubeconfig
kind get kubeconfig --name functional-test-hub > ${HUB_KUBECONFIG_DIR}/hub_kind_kubeconfig.yaml

#replace the 127.0.0.1 by Host IP
os=$(uname)
if [[ "$os" = Darwin ]]; then
  ip=$(ipconfig getifaddr en0)
  sed -i '.bak' "s/127.0.0.1/$ip/g" ${HUB_KUBECONFIG_DIR}/hub_kind_kubeconfig.yaml
else
  ip=$(hostname -I)
  sed -i "s/127.0.0.1/$ip/g" ${HUB_KUBECONFIG_DIR}/hub_kind_kubeconfig.yaml
fi

echo "creating managed cluster"
kind create cluster --name functional-test --config "${FUNCT_TEST_TMPDIR}/kind-config/kind-config.yaml"

# setup kubeconfig
kind get kubeconfig --name functional-test > ${KIND_KUBECONFIG}
cp ${KIND_KUBECONFIG} ${HUB_KUBECONFIG_DIR}/hub_kind_kubeconfig.yaml

# load image if possible
kind load docker-image ${DOCKER_IMAGE_AND_TAG} --name=functional-test -v 99 || echo "failed to load image locally, will use imagePullSecret"

# create namespace

echo "install cluster"
# setup cluster
make kind-cluster-setup

for dir in overlays/test/* ; do
  echo ">>>>>>>>>>>>>>>Executing test: $dir"

  # install klusterlet-addon-lease-controller
  echo "install managedcluster-import-controller"
  kubectl apply -k "$dir" --dry-run=true -o yaml | sed "s|REPLACE_IMAGE|${DOCKER_IMAGE_AND_TAG}|g" | kubectl apply -f -

  echo "install imagePullSecret"
  kubectl create secret -n open-cluster-management-agent-addon docker-registry multiclusterhub-operator-pull-secret --docker-server=quay.io --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS}

  # patch image
  echo "Wait rollout"
  kubectl rollout status -n open-cluster-management-agent-addon deployment klusterlet-addon-lease-controller --timeout=90s
  
  echo "run functional test..."
  make functional-test

  echo "remove deployment"
  kubectl delete -k "$dir"

done;

echo "delete managed cluster"
kind delete cluster --name functional-test

echo "delete hub cluster"
kind delete cluster --name functional-test-hub

if [ `find $FUNCT_TEST_COVERAGE -prune -empty 2>/dev/null` ]; then
  echo "no coverage files found. skipping"
else
  echo "merging coverage files"
  # report coverage if has any coverage files
  # rm -rf "${FUNCT_TEST_COVERAGE}"
  # mkdir -p "${FUNCT_TEST_COVERAGE}"

  # cp "$FUNCT_TEST_TMPDIR/output/"* "${FUNCT_TEST_COVERAGE}/"
  # ls -l "${FUNCT_TEST_COVERAGE}/"

  gocovmerge "${FUNCT_TEST_COVERAGE}/"* >> "${FUNCT_TEST_COVERAGE}/cover-functional.out"
  COVERAGE=$(go tool cover -func="${FUNCT_TEST_COVERAGE}/cover-functional.out" | grep "total:" | awk '{ print $3 }' | sed 's/[][()><%]/ /g')
  echo "-------------------------------------------------------------------------"
  echo "TOTAL COVERAGE IS ${COVERAGE}%"
  echo "-------------------------------------------------------------------------"

  go tool cover -html "${FUNCT_TEST_COVERAGE}/cover-functional.out" -o ${PROJECT_DIR}/test/functional/coverage/cover-functional.html
fi
