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

### Install Ephemeral-Access UI extension

To enable the UI extension, you must mount the React component in the Argo CD API server and deploy global variables as a configmap.
This process can be automated by using the [argocd-extension-installer v.0.06](https://github.com/argoproj-labs/argocd-extension-installer). This installation method will run an init container that will download, extract and place the file in the correct location.


##### Customize the extension to allow the argocd ui display the extension based on the application labels. 

 
- patch the application to use specific label key and value.
    ```bash
    argocd app patch myapplication --patch '{"metadata": {"labels": {"labelKey": "labelValue"}}}' --type merge
    ````


- Define the  variables in the configmap below:
    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: ephemeral-access-cm
    data:
      extension.name: 'ephemeral-access'
      extension.url: 'https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/v0.0.6/extension.tar'
      extension.version: 'v0.0.6'
      extension.js_vars : |
        {
          # Define the label key/value specific to  
         "EPHEMERAL_ACCESS_LABEL_KEY": "<labelKey>",
         "EPHEMERAL_ACCESS_LABEL_VALUE": "<labelValue>",
         "EPHEMERAL_ACCESS_MAIN_BANNER": "<message to end user>",
         "EPHEMERAL_ACCESS_MAIN_BANNER_ADDITIONAL_INFO_LINK": "<Link to docs for further readme>",
         "EPHEMERAL_ACCESS_CHANGE_REQUEST_URL": "<Link to external changes request form>"
        }
    
        
    ```


- The yaml  below is an example of how to define a kustomize patch to install this UI extension:

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
              - name: EXTENSION_NAME
                valueFrom:
                  configMapKeyRef:
                    key: ephemeral-access-cm
              - name: EXTENSION_URL
                valueFrom:
                  configMapKeyRef:
                    key: extension.url
                    name: ephemeral-access-cm
              - name: EXTENSION_Version
                valueFrom:
                  configMapKeyRef:
                    key: extension.version
                    name: ephemeral-access-cm
              - name: EXTENSION_JS_VARS
                valueFrom:
                 configMapKeyRef:
                  key: extension.js_vars
                  name: ephemeral-access-cm
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