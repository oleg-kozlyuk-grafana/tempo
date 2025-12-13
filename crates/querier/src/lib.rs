// Proto modules
pub mod httpgrpc {
    tonic::include_proto!("httpgrpc");
}

pub mod frontend {
    tonic::include_proto!("frontend");
}

pub mod opentelemetry {
    pub mod proto {
        pub mod common {
            pub mod v1 {
                tonic::include_proto!("opentelemetry.proto.common.v1");
            }
        }
        pub mod resource {
            pub mod v1 {
                tonic::include_proto!("opentelemetry.proto.resource.v1");
            }
        }
        pub mod trace {
            pub mod v1 {
                tonic::include_proto!("opentelemetry.proto.trace.v1");
            }
        }
    }
}

pub mod tempopb {
    tonic::include_proto!("tempopb");
}

// Public modules
pub mod config;
pub mod error;
pub mod frontend_processor;
pub mod http;
pub mod processor_manager;
pub mod querier_worker;
pub mod query_executor;

// Re-exports for convenience
pub use config::WorkerConfig;
pub use error::{QuerierError, Result};
pub use frontend_processor::FrontendProcessor;
pub use processor_manager::ProcessorManager;
pub use querier_worker::QuerierWorker;
pub use query_executor::{QueryExecutor, SearchParams};
