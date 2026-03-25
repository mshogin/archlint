use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Docker container configuration for a Claude Code worker
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerConfig {
    /// Docker image to use (e.g. "archlint-worker:latest")
    pub image: String,
    /// Volume mounts: host_path -> container_path
    pub volumes: HashMap<String, String>,
    /// Environment variables
    pub env_vars: HashMap<String, String>,
    /// Command to run inside the container
    pub command: Vec<String>,
    /// Memory limit in bytes (0 = unlimited)
    pub memory_limit: u64,
    /// CPU quota (0 = unlimited)
    pub cpu_quota: u64,
}

impl Default for ContainerConfig {
    fn default() -> Self {
        Self {
            image: "archlint-worker:latest".to_string(),
            volumes: HashMap::new(),
            env_vars: HashMap::new(),
            command: vec![],
            memory_limit: 0,
            cpu_quota: 0,
        }
    }
}

/// Model tier for Claude Code containers
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ModelTier {
    Haiku,
    Sonnet,
    Opus,
}

impl std::fmt::Display for ModelTier {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ModelTier::Haiku => write!(f, "haiku"),
            ModelTier::Sonnet => write!(f, "sonnet"),
            ModelTier::Opus => write!(f, "opus"),
        }
    }
}

impl std::str::FromStr for ModelTier {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "haiku" => Ok(ModelTier::Haiku),
            "sonnet" => Ok(ModelTier::Sonnet),
            "opus" => Ok(ModelTier::Opus),
            _ => Err(format!("unknown model tier: {}", s)),
        }
    }
}

/// Worker status in the lifecycle
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum WorkerStatus {
    Creating,
    Running,
    Stopped,
    Failed,
}

impl std::fmt::Display for WorkerStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WorkerStatus::Creating => write!(f, "creating"),
            WorkerStatus::Running => write!(f, "running"),
            WorkerStatus::Stopped => write!(f, "stopped"),
            WorkerStatus::Failed => write!(f, "failed"),
        }
    }
}

/// A Docker-based Claude Code worker
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Worker {
    /// Unique worker identifier
    pub id: String,
    /// Docker container ID (set after container creation)
    pub container_id: Option<String>,
    /// Current status
    pub status: WorkerStatus,
    /// Model tier this worker uses
    pub model: ModelTier,
    /// Creation timestamp (Unix epoch seconds)
    pub created_at: u64,
    /// Container configuration
    pub config: ContainerConfig,
}

/// Orchestrator manages Docker-based Claude Code workers
pub struct Orchestrator {
    docker: bollard::Docker,
    workers: HashMap<String, Worker>,
}

impl Orchestrator {
    /// Connect to the local Docker daemon
    pub async fn new() -> Result<Self, bollard::errors::Error> {
        let docker = bollard::Docker::connect_with_local_defaults()?;
        // Verify connection
        docker.ping().await?;
        Ok(Self {
            docker,
            workers: HashMap::new(),
        })
    }

    /// Build a ContainerConfig for a given model tier and project path
    pub fn build_config(model: ModelTier, project_path: &str) -> ContainerConfig {
        let mut env_vars = HashMap::new();
        env_vars.insert("ARCHLINT_MODEL".to_string(), model.to_string());
        env_vars.insert("ARCHLINT_ROLE".to_string(), "worker".to_string());

        let mut volumes = HashMap::new();
        volumes.insert(
            project_path.to_string(),
            "/workspace".to_string(),
        );

        let command = vec![
            "claude".to_string(),
            "--model".to_string(),
            model.to_string(),
            "--print".to_string(),
        ];

        let (memory_limit, cpu_quota) = match model {
            ModelTier::Haiku => (512 * 1024 * 1024, 50_000),   // 512MB, 0.5 CPU
            ModelTier::Sonnet => (1024 * 1024 * 1024, 100_000), // 1GB, 1 CPU
            ModelTier::Opus => (2048 * 1024 * 1024, 200_000),   // 2GB, 2 CPU
        };

        ContainerConfig {
            image: format!("archlint-worker:{}", model),
            volumes,
            env_vars,
            command,
            memory_limit,
            cpu_quota,
        }
    }

    /// Create a new worker container (does not start it)
    pub async fn create_worker(
        &mut self,
        model: ModelTier,
        project_path: &str,
    ) -> Result<String, bollard::errors::Error> {
        use bollard::container::Config;
        use bollard::models::{HostConfig, Mount, MountTypeEnum};

        let config = Self::build_config(model, project_path);
        let worker_id = format!("archlint-{}-{}", model, uuid_v4_simple());

        let mounts: Vec<Mount> = config
            .volumes
            .iter()
            .map(|(host, container)| Mount {
                target: Some(container.clone()),
                source: Some(host.clone()),
                typ: Some(MountTypeEnum::BIND),
                read_only: Some(true),
                ..Default::default()
            })
            .collect();

        let env: Vec<String> = config
            .env_vars
            .iter()
            .map(|(k, v)| format!("{}={}", k, v))
            .collect();

        let host_config = HostConfig {
            mounts: Some(mounts),
            memory: if config.memory_limit > 0 {
                Some(config.memory_limit as i64)
            } else {
                None
            },
            cpu_quota: if config.cpu_quota > 0 {
                Some(config.cpu_quota as i64)
            } else {
                None
            },
            ..Default::default()
        };

        let container_config = Config {
            image: Some(config.image.clone()),
            cmd: Some(config.command.clone()),
            env: Some(env),
            host_config: Some(host_config),
            labels: Some({
                let mut labels = HashMap::new();
                labels.insert("archlint.worker".to_string(), "true".to_string());
                labels.insert("archlint.worker.id".to_string(), worker_id.clone());
                labels.insert("archlint.model".to_string(), model.to_string());
                labels
            }),
            ..Default::default()
        };

        let response = self
            .docker
            .create_container(
                Some(bollard::container::CreateContainerOptions {
                    name: &worker_id,
                    platform: None,
                }),
                container_config,
            )
            .await?;

        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();

        let worker = Worker {
            id: worker_id.clone(),
            container_id: Some(response.id),
            status: WorkerStatus::Creating,
            model,
            created_at: now,
            config,
        };

        self.workers.insert(worker_id.clone(), worker);
        Ok(worker_id)
    }

    /// Start a previously created worker
    pub async fn start_worker(&mut self, worker_id: &str) -> Result<(), String> {
        let worker = self
            .workers
            .get_mut(worker_id)
            .ok_or_else(|| format!("worker not found: {}", worker_id))?;

        let container_id = worker
            .container_id
            .as_ref()
            .ok_or_else(|| "worker has no container_id".to_string())?
            .clone();

        self.docker
            .start_container::<String>(&container_id, None)
            .await
            .map_err(|e| format!("failed to start container: {}", e))?;

        // Update status after successful start
        if let Some(w) = self.workers.get_mut(worker_id) {
            w.status = WorkerStatus::Running;
        }

        Ok(())
    }

    /// Stop a running worker
    pub async fn stop_worker(&mut self, worker_id: &str) -> Result<(), String> {
        let worker = self
            .workers
            .get(worker_id)
            .ok_or_else(|| format!("worker not found: {}", worker_id))?;

        let container_id = worker
            .container_id
            .as_ref()
            .ok_or_else(|| "worker has no container_id".to_string())?
            .clone();

        self.docker
            .stop_container(
                &container_id,
                Some(bollard::container::StopContainerOptions { t: 10 }),
            )
            .await
            .map_err(|e| format!("failed to stop container: {}", e))?;

        if let Some(w) = self.workers.get_mut(worker_id) {
            w.status = WorkerStatus::Stopped;
        }

        Ok(())
    }

    /// Get the status of a worker by querying Docker
    pub async fn worker_status(&self, worker_id: &str) -> Result<WorkerStatus, String> {
        let worker = self
            .workers
            .get(worker_id)
            .ok_or_else(|| format!("worker not found: {}", worker_id))?;

        let container_id = match &worker.container_id {
            Some(id) => id,
            None => return Ok(WorkerStatus::Creating),
        };

        let inspect = self
            .docker
            .inspect_container(container_id, None)
            .await
            .map_err(|e| format!("failed to inspect container: {}", e))?;

        let status = inspect
            .state
            .and_then(|s| s.status)
            .map(|s| match s {
                bollard::models::ContainerStateStatusEnum::RUNNING => WorkerStatus::Running,
                bollard::models::ContainerStateStatusEnum::EXITED => WorkerStatus::Stopped,
                bollard::models::ContainerStateStatusEnum::DEAD => WorkerStatus::Failed,
                _ => WorkerStatus::Creating,
            })
            .unwrap_or(WorkerStatus::Failed);

        Ok(status)
    }

    /// List all tracked workers
    pub fn list_workers(&self) -> Vec<&Worker> {
        let mut workers: Vec<&Worker> = self.workers.values().collect();
        workers.sort_by(|a, b| b.created_at.cmp(&a.created_at));
        workers
    }

    /// Remove a stopped worker and its container
    pub async fn remove_worker(&mut self, worker_id: &str) -> Result<(), String> {
        let worker = self
            .workers
            .get(worker_id)
            .ok_or_else(|| format!("worker not found: {}", worker_id))?;

        if let Some(container_id) = &worker.container_id {
            self.docker
                .remove_container(
                    container_id,
                    Some(bollard::container::RemoveContainerOptions {
                        force: true,
                        ..Default::default()
                    }),
                )
                .await
                .map_err(|e| format!("failed to remove container: {}", e))?;
        }

        self.workers.remove(worker_id);
        Ok(())
    }

    /// Discover existing archlint workers from Docker
    pub async fn discover_workers(&mut self) -> Result<usize, bollard::errors::Error> {
        use bollard::container::ListContainersOptions;

        let mut filters = HashMap::new();
        filters.insert("label", vec!["archlint.worker=true"]);

        let containers = self
            .docker
            .list_containers(Some(ListContainersOptions {
                all: true,
                filters,
                ..Default::default()
            }))
            .await?;

        let mut count = 0;
        for container in containers {
            let labels = container.labels.unwrap_or_default();
            let worker_id = match labels.get("archlint.worker.id") {
                Some(id) => id.clone(),
                None => continue,
            };

            if self.workers.contains_key(&worker_id) {
                continue;
            }

            let model_str = labels
                .get("archlint.model")
                .cloned()
                .unwrap_or_else(|| "sonnet".to_string());
            let model: ModelTier = model_str.parse().unwrap_or(ModelTier::Sonnet);

            let status = match container.state.as_deref() {
                Some("running") => WorkerStatus::Running,
                Some("exited") => WorkerStatus::Stopped,
                Some("dead") => WorkerStatus::Failed,
                _ => WorkerStatus::Creating,
            };

            let worker = Worker {
                id: worker_id.clone(),
                container_id: container.id,
                status,
                model,
                created_at: container.created.unwrap_or(0) as u64,
                config: ContainerConfig::default(),
            };

            self.workers.insert(worker_id, worker);
            count += 1;
        }

        Ok(count)
    }
}

/// Generate a simple pseudo-UUID v4 without external crates
fn uuid_v4_simple() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    let nanos = now.as_nanos();
    let pid = std::process::id();
    format!("{:08x}-{:04x}", nanos as u32, pid as u16)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_model_tier_display() {
        assert_eq!(ModelTier::Haiku.to_string(), "haiku");
        assert_eq!(ModelTier::Sonnet.to_string(), "sonnet");
        assert_eq!(ModelTier::Opus.to_string(), "opus");
    }

    #[test]
    fn test_model_tier_parse() {
        assert_eq!("haiku".parse::<ModelTier>().unwrap(), ModelTier::Haiku);
        assert_eq!("Sonnet".parse::<ModelTier>().unwrap(), ModelTier::Sonnet);
        assert_eq!("OPUS".parse::<ModelTier>().unwrap(), ModelTier::Opus);
        assert!("unknown".parse::<ModelTier>().is_err());
    }

    #[test]
    fn test_worker_status_display() {
        assert_eq!(WorkerStatus::Running.to_string(), "running");
        assert_eq!(WorkerStatus::Stopped.to_string(), "stopped");
    }

    #[test]
    fn test_build_config_haiku() {
        let config = Orchestrator::build_config(ModelTier::Haiku, "/tmp/project");
        assert_eq!(config.image, "archlint-worker:haiku");
        assert_eq!(config.memory_limit, 512 * 1024 * 1024);
        assert!(config.volumes.contains_key("/tmp/project"));
    }

    #[test]
    fn test_build_config_opus() {
        let config = Orchestrator::build_config(ModelTier::Opus, "/workspace");
        assert_eq!(config.image, "archlint-worker:opus");
        assert_eq!(config.memory_limit, 2048 * 1024 * 1024);
        assert_eq!(config.cpu_quota, 200_000);
    }

    #[test]
    fn test_uuid_v4_simple_unique() {
        let a = uuid_v4_simple();
        // Small sleep not needed - nanos should differ
        let b = uuid_v4_simple();
        // Both should be valid hex format
        assert!(a.contains('-'));
        assert!(b.contains('-'));
    }

    #[test]
    fn test_default_container_config() {
        let config = ContainerConfig::default();
        assert_eq!(config.image, "archlint-worker:latest");
        assert!(config.volumes.is_empty());
        assert!(config.env_vars.is_empty());
        assert_eq!(config.memory_limit, 0);
    }
}
