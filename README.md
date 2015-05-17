## Introduction

The goal of this project is to automatically create/update Vulcanproxy rules for all the Kubernetes services currently running.

Kubernetes is becoming more and more popular to manage Linux containers at scale and provide some very advanced capabilities, like managing rolling upgrades easily.

Kubernetes is assigning a different IP address for each service to avoid any port collision, but, in the past, it was impossible to reach these IP addresses from the outside world.

Now, Kubernetes services can be used, but they don't provide all the nice load balancing features of Vulcand.

Vulcand is a reverse proxy for HTTP API management and microservices.

And Vulcand is watching etcd to automatically detect new rules it needs to implement, so you don't need to reload any service. Simply add the right keys in etcd and your service/application becomes available from the outside world.

More information available at http://www.vulcanproxy.com

This tool isn't "production ready" and was developed to show end to end automation.

### Configuration

Before running this tool, you need to specify for what Kubernetes services Vulcanproxy rules should be created.

First, you need to create a root directory in etcd:

```
etcdctl mkdir /kubernetes-vulcan
```

Then, you need to create a subdirectory using the name of the Kubernetes service:

```
etcdctl mkdir /kubernetes-vulcan/app1
```

Finally, you need to indicate the Vulcanproxy frontends and backends to use for this Kubernetes service:

```
etcdctl set /mesos-vulcan/app1/frontend f1
etcdctl set /mesos-vulcan/app1/backend b1
```

### Run

This tool will:

- determine what Kubernetes services are running without a corresponding vulcand rule in etcd and create the missing rules
- determine what vulcand rules exist in etdc for Kubernetes services which aren't running anymore and delete them

The Syntax is pretty simple:

```
./kubernetes-vulcan -KubernetesEndPoint=http://<Kubernetes API server IP>:8080/v2/apps -EtcdEndPoint=http://<Etcd server IP>:4001/v2 -EtcdRootKey=/kubernetes-vulcan
```

You can schedule this tool to run every minute to automatically make your Kubernetes services externally available

# Licensing

Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
