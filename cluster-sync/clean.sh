#!/bin/bash -e
  
source ./hack/build/config.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh

echo "Cleaning up ..."

OPERATOR_CR_MANIFEST=./_out/manifests/release/cdi-cr.yaml
OPERATOR_MANIFEST=./_out/manifests/release/cdi-operator.yaml
LABELS=("operator.cdi.kubevirt.io" "cdi.kubevirt.io" "prometheus.cdi.kubevirt.io")
NAMESPACES=(default kube-system cdi)

set +e
_kubectl patch cdi cdi --type=json -p '[{ "op": "remove", "path": "/metadata/finalizers" }]'
set -e

if [ -f "${OPERATOR_CR_MANIFEST}" ]; then
	echo "Cleaning CR object ..."
    if _kubectl get crd cdis.cdi.kubevirt.io ; then
        _kubectl delete --ignore-not-found -f "${OPERATOR_CR_MANIFEST}"
        _kubectl wait cdis.cdi.kubevirt.io/${CR_NAME} --for=delete | echo "this is fine"
    fi
fi

if [ -f "${OPERATOR_MANIFEST}" ]; then
	echo "Deleting operator ..."
    _kubectl delete --ignore-not-found -f "${OPERATOR_MANIFEST}"
fi

# Everything should be deleted by now, but just to be sure
for n in ${NAMESPACES[@]}; do
  for label in ${LABELS[@]}; do
    _kubectl -n ${n} delete deployment -l ${label}
    _kubectl -n ${n} delete services -l ${label}
    _kubectl -n ${n} delete secrets -l ${label}
    _kubectl -n ${n} delete configmaps -l ${label}
    _kubectl -n ${n} delete pvc -l ${label}
    _kubectl -n ${n} delete pods -l ${label}
    _kubectl -n ${n} delete rolebinding -l ${label}
    _kubectl -n ${n} delete roles -l ${label}
    _kubectl -n ${n} delete serviceaccounts -l ${label}
  done
done

for label in ${LABELS[@]}; do
    _kubectl delete pv -l ${label}
    _kubectl delete mutatingwebhookconfiguration -l ${label}
    _kubectl delete validatingwebhookconfiguration -l ${label}
    _kubectl delete clusterrolebinding -l ${label}
    _kubectl delete clusterroles -l ${label}
    _kubectl delete customresourcedefinitions -l ${label}
    _kubectl get apiservices -l ${label} -o=custom-columns=NAME:.metadata.name,FINALIZERS:.metadata.finalizers --no-headers | grep foregroundDeletion | while read p; do
        arr=($p)
        name="${arr[0]}"
        _kubectl -n ${i} patch apiservices $name --type=json -p '[{ "op": "remove", "path": "/metadata/finalizers" }]'
    done
done


if [ -n "$(_kubectl get ns | grep "cdi ")" ]; then
    echo "Clean cdi namespace"
    _kubectl delete ns $NAMESPACE

    start_time=0
    sample=10
    timeout=120 
    echo "Waiting for cdi namespace to disappear ..."
    while [ -n "$(_kubectl get ns | grep "$NAMESPACE ")" ]; do
        sleep $sample
        start_time=$((current_time + sample))
        if [[ $current_time -gt $timeout ]]; then
            exit 1
        fi
    done
fi
sleep 2
echo "Done"
