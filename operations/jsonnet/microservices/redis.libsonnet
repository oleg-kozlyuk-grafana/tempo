{
  local k = import 'ksonnet-util/kausal.libsonnet',
  local container = k.core.v1.container,
  local containerPort = k.core.v1.containerPort,
  local statefulSet = k.apps.v1.statefulSet,
  local service = k.core.v1.service,

  redis_container::
    container.new('redis', $._images.redis) +
    container.withPorts([containerPort.new('redis', 6379)]) +
    container.withArgs([
      '--maxmemory',
      $._config.redis.maxmemory,
      '--maxmemory-policy',
      $._config.redis.maxmemory_policy,
    ]) +
    $.util.resourcesRequests('1', '4Gi') +
    $.util.resourcesLimits('2', '6Gi'),

  redis_exporter::
    container.new('exporter', $._images.redisExporter) +
    container.withPorts([containerPort.new('http-metrics', 9121)]) +
    container.withArgs([
      '--redis.addr=redis://localhost:6379',
    ]),

  redis_statefulset:
    if $._config.redis.enabled then
      statefulSet.new('redis', $._config.redis.replicas, [
        $.redis_container,
        $.redis_exporter,
      ], []) +
      statefulSet.mixin.spec.withServiceName('redis') +
      k.util.antiAffinityStatefulSet,

  redis_service:
    if $._config.redis.enabled then
      k.util.serviceFor($.redis_statefulset) +
      service.mixin.spec.withClusterIp('None'),

  redis_pdb:
    if $._config.redis.enabled then
      $.pdbForController($.redis_statefulset, 'redis'),
}
