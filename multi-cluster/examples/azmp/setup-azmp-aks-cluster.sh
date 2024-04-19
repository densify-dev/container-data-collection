#!/bin/bash

CLUSTER_NAME=<my_azure_aks_cluster_name>
RESOURCE_GROUP=<my_azure_aks_cluster_resourcegroup>
AZMP_ALREADY_ENABLED=1
ADD_TO_KUBECONFIG=1

if [[ ${ADD_TO_KUBECONFIG} -eq 1 ]]; then
    az aks get-credentials -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP} --admin
fi

if [[ ${AZMP_ALREADY_ENABLED} -eq 1 ]]; then
    az aks update --disable-azure-monitor-metrics -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP}
fi

kubectl create -f ./ama-metrics-settings-configmap.yaml

az aks update --enable-azure-monitor-metrics -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP} --ksm-metric-labels-allow-list 'nodes=[*],namespaces=[*],pods=[*],deployments=[*],replicasets=[*],daemonsets=[*],statefulsets=[*],jobs=[*],cronjobs=[*],horizontalpodautoscalers=[*]' --ksm-metric-annotations-allow-list 'namespaces=[*]'
