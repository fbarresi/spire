# Agent plugin: WorkloadAttestor "windows_service"

The `windows_service` plugin generates selectors based on local running service information of the workloads calling the agent.

This plugin does not require any configuration options.

General selectors:

| Selector                       | Value                                                                                                |
|--------------------------------|------------------------------------------------------------------------------------------------------|
| `windows_service:name`         | The service name associated to the workload (e.g. `windows_service:name:nginx`)                      |
| `windows_service:display_name` | The service display name associated to the workload (e.g. `windows_service:display_name:FancyNginx`) |

A sample configuration:

```hcl
    WorkloadAttestor "windows_service" {
    }
```

## Platform support

This plugin is only supported on Windows systems.

