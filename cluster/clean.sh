#!/bin/bash -e
  
source ./cluster/gocli.sh
source ./hack/build/config.sh
source ./cluster/${KUBEVIRT_PROVIDER}/provider.sh

echo "Cleaning up ..."

OPERATOR_CR_MANIFEST=./_out/manifests/release/cdi-cr.yaml
OPERATOR_MANIFEST=./_out/manifests/release/cdi-operator.yaml

if [ -f "${OPERATOR_CR_MANIFEST}" ]; then
    if _kubectl get crd cdis.cdi.kubevirt.io ; then
        _kubectl delete --ignore-not-found -f "${OPERATOR_CR_MANIFEST}"
        _kubectl wait cdis.cdi.kubevirt.io/cdi --for=delete | echo "this is fine"
    fi
fi

if [ -f "${OPERATOR_MANIFEST}" ]; then
    _kubectl delete --ignore-not-found -f "${OPERATOR_MANIFEST}"
fi

# Everything should be deleted by now, but just to be sure
namespaces=(default kube-system)
for i in ${namespaces[@]}; do
    _kubectl -n ${i} delete deployment -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete services -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete secrets -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete configmaps -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete pvc -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete pods -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete rolebinding -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete roles -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
    _kubectl -n ${i} delete serviceaccounts -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
done

_kubectl delete pv -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
_kubectl delete validatingwebhookconfiguration -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
_kubectl delete clusterrolebinding -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
_kubectl delete clusterroles -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'
_kubectl delete customresourcedefinitions -l 'operator.cdi.kubevirt.io' -l 'cdi.kubevirt.io'

sleep 2

echo "Done"
