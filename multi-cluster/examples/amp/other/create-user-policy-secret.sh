#!/bin/bash

# AWS region, mandatory
AWS_REGION=<AWS region>
# AWS IAM user name, mandatory - can be existing user or user to create
AWS_USER=<AWS IAM user>
# If AWS_USER does not exist, password is mandatory - and should be quoted with single quotes; otherwise should be omitted
AWS_PASSWORD=''
# If AWS_USER exists, mandatory; otherwise should be omitted
AWS_ACCESS_KEY=
# If AWS_USER exists, mandatory; otherwise should be omitted
AWS_SECRET_KEY=
# Policy to create if AMP_QUERY_POLICY_ARN is not specified
AMP_QUERY_POLICY=AMPQueryPolicy
# Policy ARN, mandatory if the policy exists already. the value will then be:
# arn:aws:iam::<AWS account Id>:policy/AMPQueryPolicy
AMP_QUERY_POLICY_ARN=

#
# Set up the permission policy that grants query permissions for all AMP workspaces
#
cat <<EOF > PermissionPolicyQuery.json
{
  "Version": "2012-10-17",
   "Statement": [
       {"Effect": "Allow",
        "Action": [
           "aps:QueryMetrics",
           "aps:GetSeries", 
           "aps:GetLabels",
           "aps:GetMetricMetadata"
        ], 
        "Resource": "*"
      }
   ]
}
EOF

if [[ -z "${AWS_USER}" ]]; then
  echo "AWS IAM user not specified"
  exit 1
fi

if [[ -z "${AWS_REGION}" ]]; then
  echo "AWS region not specified"
  exit 2
fi


keys_specified=0
if [[ -n "${AWS_ACCESS_KEY}" ]]; then
  ((keys_specified++))
fi

if [[ -n "${AWS_SECRET_KEY}" ]]; then
  ((keys_specified++))
fi

# check if user exists and accordingly parameters
aws iam get-user --user-name ${AWS_USER} &>/dev/null
if [[ $? -eq 0 ]]; then
  if [[ -n ${AWS_PASSWORD} ]]; then
    echo "AWS IAM user ${AWS_USER} exists, but password is provided"
    exit 3
  elif [[ ${keys_specified} -lt 2 ]]; then
    echo "AWS IAM user ${AWS_USER} exists, but access key or secret key is not provided"
    exit 4
  fi
else
  if [[ -z ${AWS_PASSWORD} ]]; then
    echo "AWS IAM user ${AWS_USER} does not exist, but password is not provided for user creation"
    exit 5
  elif [[ ${keys_specified} -gt 0 ]]; then
    echo "AWS IAM user ${AWS_USER} does not exist, but access key or secret key is provided"
    exit 6
  else
    aws iam create-user --user-name ${AWS_USER}
    if [[ $? -ne 0 ]]; then
      echo "Failed to create AWS IAM user ${AWS_USER}"
      exit 7
    fi
    aws iam create-login-profile --user-name ${AWS_USER} --password ${AWS_PASSWORD}
    if [[ $? -ne 0 ]]; then
      echo "Failed to set password for AWS IAM user ${AWS_USER}"
      exit 8
    fi
    access_key=$(aws iam create-access-key --user-name ${AWS_USER})
    if [[ $? -eq 0 ]]; then
      AWS_ACCESS_KEY=$(echo ${access_key} | jq -r '.AccessKey.AccessKeyId')
      AWS_SECRET_KEY=$(echo ${access_key} | jq -r '.AccessKey.SecretAccessKey')
    else
      echo "Failed to create access key for AWS IAM user ${AWS_USER}"
      exit 9
    fi
  fi
fi

if [[ -z ${AMP_QUERY_POLICY_ARN} ]]; then
  AMP_QUERY_POLICY_ARN=$(aws iam create-policy --policy-name ${AMP_QUERY_POLICY} \
  --policy-document file://PermissionPolicyQuery.json \
  --query 'Policy.Arn' --output text)
  if [[ $? -ne 0 ]]; then
    echo "Failed to create AWS IAM policy ${AMP_QUERY_POLICY}"
    exit 10
  fi
fi

aws iam attach-user-policy --user-name ${AWS_USER} --policy-arn ${AMP_QUERY_POLICY_ARN}
if [[ $? -ne 0 ]]; then
  echo "Failed to attach policy with ARN ${AMP_QUERY_POLICY_ARN} to AWS IAM user ${AWS_USER}"
  exit 11
fi

aws_dir=aws-files
rm -rf ${aws_dir}
mkdir -p ${aws_dir}
config_file=${aws_dir}/config
cat <<EOF > ${config_file}
[default]
region = ${AWS_REGION}
output = json
EOF
config_base64=$(cat ${config_file} | base64 -w 0)

cred_file=${aws_dir}/credentials
cat <<EOF > ${cred_file}
[default]
aws_access_key_id = ${AWS_ACCESS_KEY}
aws_secret_access_key = ${AWS_SECRET_KEY}
EOF
cred_base64=$(cat ${cred_file} | base64 -w 0)

cat <<EOF > aws-secret.yaml
apiVersion : v1
kind : Secret
metadata :
  name : aws
data : 
  config : ${config_base64}
  credentials : ${cred_base64}
EOF

# access key, secret key provided - can delete the generated AWS files
if [[ ${keys_specified} -eq 2 ]]; then
  rm -rf ${aws_dir}
fi

rm -rf *.json
