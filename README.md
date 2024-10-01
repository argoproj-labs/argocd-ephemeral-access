# argocd-ephemeral-access

A kubernetes controller to manage Argo CD temporary access

### Development

Build the image locally

```bash
IMAGE_NAMESPACE="my.company.com/argoproj-labs" IMAGE_TAG="$(git rev-parse --abbrev-ref HEAD)" make docker-build
```

Install in kubernetes cluster

```bash
IMAGE_NAMESPACE="my.company.com/argoproj-labs" IMAGE_TAG="$(git rev-parse --abbrev-ref HEAD)" make deploy-local
```

### Install Argocd-Ephemeral-Access UI extension

To enable the UI extension, you must mount the React component in the Argo CD API server and deploy global variables as a configmap. These variables define the label and key that the
`permission` button will use to display in the Argo CD application UI.
This process can be automated by using the argocd-extension-installer. This installation method will run an init container that will download, extract and place the file in the correct location.


Define the labels key and value:
```text
window.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY=<provide the label key>
window.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_VALUE=<provide the label value>

Additional configurations: Define the main banner message and url to provide the information about the change request process.
window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER=<provide the main banner message>
window.GLOBAL_ARGOCD_ACCESS_EXT_CHANGE_REQUEST_URL 
```

The yaml below is an example  to define the global vars configmap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-ephemeral-access-global-vars-cm
data:
  argocd-ephemeral-access-global-vars.js: |
    ((window) => {
      // Set global variables
      window.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_KEY = <provide the label key>;
      window.GLOBAL_ARGOCD_ACCESS_EXT_LABEL_VALUE = <provide the label value>
      // Change request banner message as per the requirement
      window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER = "All production changes require an associated change request. Click the REQUEST ACCESS " +
        "button above to automatically create a change request associated with your user,"
      window.GLOBAL_ARGOCD_ACCESS_EXT_MAIN_BANNER_ADDITIONAL_INFO_LINK = "https://additional-info-link.com";
      window.GLOBAL_ARGOCD_ACCESS_EXT_CHANGE_REQUEST_URL = "https://change-request-url.com";
    })(window);

```

The yaml  below is an example of how to define a kustomize patch to install this UI extension:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  template:
    spec:
      initContainers:
        - name: extension-metrics
          image: quay.io/argoprojlabs/argocd-extension-installer:v0.0.1
          env:
          - name: EXTENSION_URL
            value: https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/<Latest_release>/extension.tar.gz
          - name: EXTENSION_CHECKSUM_URL
            value: https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/<Latest_release>/extension_checksums.txt
          volumeMounts:
            - name: extensions
              mountPath: /tmp/extensions/
          securityContext:
            runAsUser: 1000
            allowPrivilegeEscalation: false
      containers:
        - name: argocd-server
          volumeMounts:
            - name: extensions
              mountPath: /tmp/extensions/
            - name: argocd-ephemeral-access-global-vars-cm
              mountPath: /tmp/extensions/
      volumes:
        - name: extensions
          emptyDir: {}
        - name: argocd-ephemeral-access-global-vars-cm
          configMap:
            name: argocd-ephemeral-access-global-vars-cm
```