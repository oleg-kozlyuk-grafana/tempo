local k = import 'ksonnet-util/kausal.libsonnet';

local container = k.core.v1.container;
local containerPort = k.core.v1.containerPort;
local envVar = k.core.v1.envVar;
local statefulSet = k.apps.v1.statefulSet;
local volumeMount = k.core.v1.volumeMount;
local pvc = k.core.v1.persistentVolumeClaim;
local service = k.core.v1.service;
local job = k.batch.v1.job;

{
  redis_container::
    container.new('redis', $._images.redis) +
    container.withArgs([
      'redis-server',
      '--cluster-enabled', 'yes',
      '--cluster-config-file', '/data/nodes.conf',
      '--cluster-node-timeout', '5000',
      '--maxmemory', $._config.redis.maxmemory,
      '--maxmemory-policy', $._config.redis.maxmemory_policy,
      '--bind', '0.0.0.0',
      '--port', std.toString($._config.redis.port),
    ]) +
    container.withPorts([containerPort.new('redis', $._config.redis.port)]) +
    container.withVolumeMounts([
      volumeMount.new('redis-data', '/data'),
    ]) +
    $.util.withResources($._config.redis.resources),

  redis_exporter_container::
    container.new('redis-exporter', $._images.redisExporter) +
    container.withEnv([
      envVar.new('REDIS_ADDR', 'redis://localhost:%d' % $._config.redis.port),
    ]) +
    container.withPorts([containerPort.new('metrics', 9121)]),

  local redisInitAddrs = std.join(' ', [
    'redis-%d.redis:%d' % [i, $._config.redis.port]
    for i in std.range(0, $._config.redis.replicas - 1)
  ]),

  redis_statefulset: if $._config.cache_type != 'redis' then {} else
    statefulSet.new('redis', $._config.redis.replicas, [
      $.redis_container,
      $.redis_exporter_container,
    ], [
      pvc.new('redis-data') +
      pvc.spec.withAccessModes(['ReadWriteOnce']) +
      pvc.spec.resources.withRequests({ storage: '1Gi' }),
    ]) +
    statefulSet.mixin.spec.withServiceName('redis') +
    k.util.antiAffinityStatefulSet,

  redis_service: if $._config.cache_type != 'redis' then {} else
    k.util.serviceFor($.redis_statefulset) +
    // Headless service for stable DNS names required by StatefulSet and cluster node discovery.
    service.mixin.spec.withClusterIp('None'),

  // One-time Job to bootstrap the Redis Cluster after all pods start.
  // It waits for all nodes to respond to PING, then runs cluster create.
  redis_cluster_init_job: if $._config.cache_type != 'redis' then {} else
    job.new('redis-cluster-init') +
    job.spec.template.spec.withRestartPolicy('OnFailure') +
    job.spec.template.spec.withContainers([
      container.new('redis-cluster-init', $._images.redis) +
      container.withCommand(['/bin/sh', '-c']) +
      container.withArgs([
        |||
          set -e
          for ep in %(addrs)s; do
            host="${ep%%:*}"
            port="${ep##*:}"
            until redis-cli -h "$host" -p "$port" ping; do
              echo "Waiting for $ep..."
              sleep 2
            done
          done
          redis-cli --cluster create --cluster-yes --cluster-replicas 0 %(addrs)s
        ||| % { addrs: redisInitAddrs },
      ]),
    ]),

  // VPA for Redis.
  redis_vpa: if $._config.cache_type != 'redis' then {} else $.vpaForController($.redis_statefulset, 'redis'),

  // PDB for Redis.
  redis_pdb: if $._config.cache_type != 'redis' then {} else $.pdbForController($.redis_statefulset, 'redis'),
}
