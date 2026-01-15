#!/bin/bash

excludeAfter='node_group\|container_id\|kubernetes_io\|kubernetes_feature\|hpa_\|allocatable_\|capacity_\|limits_\|requests_'
DOCS_DIR=$(dirname "${PWD}")
TEST_FILE=${DOCS_DIR}/metrics-test.txt
ALL_FILE=${DOCS_DIR}/metrics-all.txt
QUOTE="'"
process_job() {
	if [ "$#" -lt 2 ]; then
		return
	fi
	name=${1}
	file1=${DOCS_DIR}/${name}-metrics-logs.txt
	file2=${DOCS_DIR}/${name}-metrics-docs.txt
	file3=${DOCS_DIR}/${name}-metrics.txt
	file4=${DOCS_DIR}/${name}-metrics-regex.txt
	file5=${DOCS_DIR}/${name}-metrics.yaml
	rm -f ${DOCS_DIR}/${name}-metrics*.txt ${DOCS_DIR}/${name}-metrics*.yaml
	cd ${DATA_DIR}
	local prefixes=("${@:2}")
	for prefix in "${prefixes[@]}"; do
		grep -R --include log.txt -h -v '.*detected metrics.*\|.*Prometheus TSDB status.*' | \
		grep -o "\b${prefix}_[a-zA-Z0-9_]*[a-zA-Z0-9]\b" | \
		grep -v ${excludeAfter} | sort -u >> ${file1}
		sed -e 's%<br/>%%g' ${DOCS_DIR}/*.md | \
		grep -o "\b${prefix}_[a-zA-Z0-9_]*[a-zA-Z0-9]\b" | grep -v ${excludeAfter} | sort -u >> ${file2}
	done
	cd ${DOCS_DIR}
	diff ${file1} ${file2}
	cat ${file1} ${file2} | sort -u > ${file3}
	cat ${file3} >> ${ALL_FILE}
	regex=$(./build-regex.sh ${file3})
	grep -E "${regex}" ${file3} >> ${TEST_FILE}
	echo "${regex}" > ${file4}
	cat << EOF > ${file5}
          metric_relabel_configs:
            - source_labels: [__name__]
              regex: ${QUOTE}${regex}${QUOTE}
              action: keep
EOF
}

if [[ -z "${DATA_DIR}" ]]; then
	echo "env var DATA_DIR is empty"
	exit 1
fi
rm -f ${TEST_FILE} ${ALL_FILE}
process_job api-server kubernetes
process_job kubelet kubernetes
process_job cadvisor container
process_job endpoints kube node DCGM openshift
diff ${TEST_FILE} ${ALL_FILE}
wc -l ${DOCS_DIR}/*metrics*.txt
