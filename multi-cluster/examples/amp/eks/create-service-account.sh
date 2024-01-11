CLUSTER_NAME=<my_amazon_eks_clustername>
SERVICE_ACCOUNT_NAMESPACE=<my_forwarder_namespace>
SERVICE_ACCOUNT_IAM_AMP_QUERY_ROLE=amp-iamproxy-query-role
SERVICE_ACCOUNT_NAME=amp-iamproxy-query-service-account

function getRoleArn() {
  OUTPUT=$(aws iam get-role --role-name ${1} --query 'Role.Arn' --output text 2>&1)
  if [[ $? -eq 0 ]]; then
    echo ${OUTPUT}
  elif [[ -n $(grep "NoSuchEntity" <<< ${OUTPUT}) ]]; then
    echo ""
  else
    >&2 echo ${OUTPUT}
    return 1
  fi
}

SERVICE_ACCOUNT_IAM_AMP_QUERY_ROLE_ARN=$(getRoleArn ${SERVICE_ACCOUNT_IAM_AMP_QUERY_ROLE})

eksctl create iamserviceaccount --name ${SERVICE_ACCOUNT_NAME} --namespace ${SERVICE_ACCOUNT_NAMESPACE} --cluster ${CLUSTER_NAME} \
--attach-role-arn ${SERVICE_ACCOUNT_IAM_AMP_QUERY_ROLE_ARN} --approve
