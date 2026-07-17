# kubectl-cce

`kubectl-cce` is a Go kubectl plugin that starts a short-lived local reverse
proxy, adds Huawei Cloud CCE authentication, runs the real `kubectl`, and then
shuts the proxy down.

The plugin does not hard-code pods. It lets the real `kubectl` perform normal
Kubernetes discovery and resource mapping, so normal REST-style resources such
as namespaces, nodes, pods, deployments, services, configmaps, secrets,
ingresses, PVs, PVCs, jobs, cronjobs, roles, and CRDs use the same command shape
as upstream kubectl.

## Install

### From GitHub Releases (recommended)

Download the archive for your platform from
[Releases](https://github.com/pancake0001/kubectl-cce-plugin/releases),
extract it, and put the binary on your `PATH`. Rename it to `kubectl-cce`
(`kubectl-cce.exe` on Windows) so kubectl can discover it.

| OS | Arch | Archive |
| --- | --- | --- |
| Linux | amd64 | `kubectl-cce_<version>_linux_amd64.tar.gz` |
| Linux | arm64 | `kubectl-cce_<version>_linux_arm64.tar.gz` |
| Windows | amd64 | `kubectl-cce_<version>_windows_amd64.zip` |
| Windows | arm64 | `kubectl-cce_<version>_windows_arm64.zip` |

```bash
tar -xzf kubectl-cce_*_linux_amd64.tar.gz
chmod +x kubectl-cce
mv kubectl-cce /usr/local/bin/
kubectl plugin list
kubectl cce --version
```

### From source

```bash
go build -o kubectl-cce ./cmd/kubectl-cce
export PATH="$PWD:$PATH"
kubectl plugin list
```

kubectl discovers the plugin because the executable is named `kubectl-cce`.

## Configure

AK/SK is preferred when both AK and SK are present. Set the `HW_`-prefixed
variables (`HUAWEICLOUD_` and `HUAWEI_CLOUD_` spellings are accepted as
aliases):

```bash
export HW_ACCESS_KEY="your-ak"
export HW_SECRET_KEY="your-sk"
```

You can pass the cluster and region directly:

```bash
kubectl cce --cluster-id your-cluster-id --region cn-north-4 get ns
```

Or keep them in environment variables:

```bash
export CCE_CLUSTER_ID="your-cluster-id"
export HW_REGION="cn-north-4"
export HW_PROJECT_ID="your-project-id" # optional, but recommended for AK/SK
```

For a temporary AK/SK, also set:

```bash
export HW_SECURITY_TOKEN="your-security-token"
```

If Huawei Cloud returns an AK/SK authentication or signature error, enable
debug output to inspect the canonical request and string-to-sign:

```bash
export CCE_PROXY_DEBUG=1
kubectl cce get ns
```

If AK/SK is not set, the plugin can fall back to an IAM token:

```bash
export HUAWEI_IAM_TOKEN="your-iam-token"
```

You can override the target host directly:

```bash
export CCE_ENDPOINT="your-cluster-id.cce.cn-north-4.myhuaweicloud.com"
```

## Usage

```bash
kubectl cce get pods -n default
kubectl cce --cluster-id your-cluster-id --region cn-north-4 get ns
kubectl cce get pods -A
kubectl cce get ns
kubectl cce get nodes
kubectl cce get svc,deploy,cm -n default
kubectl cce get ingress -A
kubectl cce get pv,pvc -A
kubectl cce get jobs,cronjobs -A
kubectl cce get crd
kubectl cce get deployments -n default -o yaml
kubectl cce describe pod nginx -n default
kubectl cce logs nginx -n default
```

The plugin internally runs something like:

```bash
kubectl \
  --server=http://127.0.0.1:<random-port> \
  --insecure-skip-tls-verify=true \
  --kubeconfig=/dev/null \
  get pods -n default
```

## Current Limitations

This MVP targets normal REST-style kubectl calls. Streaming commands such as
`exec`, `attach`, and `port-forward` are intentionally blocked because CCE API
Gateway does not reliably pass the websocket/SPDY upgrade required by these
commands.

`logs -f` and `watch` may work depending on the CCE API gateway behavior, but
they have not been hardened yet.
