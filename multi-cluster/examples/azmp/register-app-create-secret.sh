#!/bin/bash

#AZMON_WORKSPACE_RESOURCE_URI=/subscriptions/${subscriptionId}/resourceGroups/${resourceGroupId}/providers/microsoft.monitor/accounts/${workspaceId}
AZMON_WORKSPACE_RESOURCE_URI=<my_azure_monitor_workspace_resource_URI>
DCDC=densify-container-data-collection
APP_FILE=${DCDC}.json
rm -f ${APP_FILE}
az ad sp create-for-rbac -n ${DCDC} > ${APP_FILE}
APP_ID=$(jq -r '.appId' ${APP_FILE})
az role assignment create --assignee ${APP_ID} --role 'Monitoring Data Reader' --scope ${AZMON_WORKSPACE_RESOURCE_URI}
app_base64=$(cat ${APP_FILE} | base64 -w 0)
cat <<EOF > azmon-secret.yaml
apiVersion : v1
kind : Secret
metadata :
  name : azmon
data : 
  app.json : ${app_base64}
EOF
