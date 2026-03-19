local tempo = import '../tempo.libsonnet';
tempo {
  _images+:: {},
  _config+:: {
    cluster: 'k3d',
    namespace: 'default',
    cache_type: 'redis',
    distributor+: {
      receivers: { jaeger: { protocols: { thrift_http: null } } },
    },
    metrics_generator+: {
      pvc_size: '5Gi',
      pvc_storage_class: 'local-path',
      ephemeral_storage_limit_size: '2Gi',
      ephemeral_storage_request_size: '1Gi',
    },
    backend_scheduler+: {
      pvc_size: '200Mi',
      pvc_storage_class: 'local-path',
    },
    live_store+: {
      pvc_size: '5Gi',
      pvc_storage_class: 'local-path',
    },
    backend: 's3',
    bucket: 'tempo',
    overrides_configmap_name: 'tempo-overrides',
    overrides+:: {},
  },
}
