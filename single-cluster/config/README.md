# Configuration

Use this [config.yaml](https://github.com/densify-dev/container-config/blob/main/examples/config.yaml) file (with a single cluster with no identifiers) as a template.

> **_NOTE:_**  V4 of Densify Container Data Collection is backwards-compatible and has full support for the deprecated **properties** format of the config map of versions 1-3. However, new features introduced in V4 are configurable using **yaml** format only, and new config maps should be created only using **yaml** format. The **properties** format will be removed in a feature release.

## Configuration Variables

The following tables provide the parameter names and default values for the variables used to configure the data forwarder.

The order of precedence is: command line flags, environment variables, config file (typically provided as a config map).

### Variable Names Data Collection

As of version 4.0.0, for data collection the same parameter name is used for all three configuration sources (for environment variables in CAPS).

| Setting                | Parameter Name         | Default Value                                                                                                                                                              |
|------------------------|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Config Path [^1]       | config_dir             | ./config                                                                                                                                                                   |
| Config File [^1]       | config_file            | config                                                                                                                                                                     |
| Config Type [^1]       | config_type            | properties                                                                                                                                                                 |
| Prometheus Scheme      | prometheus_protocol    | http                                                                                                                                                                       |
| Prometheus Address     | prometheus_address     |                                                                                                                                                                            |
| Prometheus Port        | prometheus_port        | 9090                                                                                                                                                                       |
| Prometheus User        | prometheus_user        |                                                                                                                                                                        |
| Prometheus Password        | prometheus_password        |                                                                                                                                                                        |
| Prometheus OAuth Token | prometheus_oauth_token |                                                                                                                                                                            |
| CA Certificate         | ca_certificate         |                                                                                                                                                                            |
| Cluster Name           | cluster_name           |                                                                                                                                                                            |
| Interval               | interval               | hours                                                                                                                                                                      |
| Interval Size          | interval_size          | 1                                                                                                                                                                          |
| History                | history                | 1                                                                                                                                                                          |
| Offset                 | offset                 | 0                                                                                                                                                                          |
| Sample Rate            | sample_rate            | 5                                                                                                                                                                          |
| Include List           | include_list           | container,node,cluster,nodegroup,quota                                                                                                                                     |
| Node Group List        | node_group_list        | label_cloud_google_com_gke_nodepool,label_eks_amazonaws_com_nodegroup,label_agentpool,label_pool_name,label_alpha_eksctl_io_nodegroup_name,label_kops_k8s_io_instancegroup |
| Debug                  | debug                  | false                                                                                                                                                                      |

[^1]: The parameters specifying the config file are only available as command line flags and environment variables, these cannot be present inside the config file itself.

### Variable Names Forwarder

As of version 4.0.0, for the forwarder the same parameter name is used for all three configuration sources (for environment variables in CAPS and with the prefix `DENSIFY_`).

| Config Setting Name  | Parameter Name | 
|--------|-------|
| Host | host |
| Scheme | protocol |
| Port | port |
| Endpoint | endpoint |
| User | user |
| Password | password | 
| Encrypted Password | epassword | 
| Proxy Host | proxyhost | 
| Proxy Port | proxyport |
| Proxy Scheme | proxyprotocol | 
| Proxy Auth | proxyauth |
| Proxy User | proxyuser |
| Proxy Password | proxypassword | 
| Encrypted Proxy Password | eproxypassword | 
| Proxy Server | proxyserver |
| Proxy Domain | proxydomain | 
| Zip File Prefix | prefix | 
| Debug | debug | 
