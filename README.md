# Gemini CLI Go Migration Plan

This document outlines the plan for migrating the Gemini CLI from TypeScript to Go.

## Status

- [x] **In Progress**
- [ ] **Complete**

## Migration Checklist

### Core Functionality

| Feature | Status | Notes |
| --- | --- | --- |
| **Argument Parsing** | ✅ **Done** | Basic argument parsing is implemented using `cobra`. |
| **Configuration Loading** | ✅ **Done** | Configuration loading from `settings.toml` (and deprecated `settings.json`) is implemented and tested. |
| **Authentication** | ✅ **Done** | `CloudShellAuthenticator` and `OAuth2Authenticator` are implemented. |
| **Interactive Mode (TUI)** | ✅ **Done** | Migrated to Go using `bubbletea`. |
| **Non-Interactive Mode** | ✅ **Done** | The core logic for handling single prompts, streaming, and multi-turn conversations is implemented and tested. |
| **Stdin Reading** | ✅ **Done** | Implemented reading from stdin when input is piped to the CLI. |
| **Command Execution** | ✅ **Done** | The core logic for sending prompts to the Gemini API, handling streaming responses, and executing tool calls is implemented. |
| **Error Handling** | 🚧 **In Progress** | Basic error handling is in place. More robust error handling is needed. |
| **Sandbox** | ✅ **Done** | Implemented using `docker`, `podman`, and `sandbox-exec` on macOS. |
| **Update Checker** | ✅ **Done** | Checks for new releases on GitHub and displays a notification in the TUI. |
| **Auto Update** | ❌ **Not Started** | Implement a mechanism to automatically update the CLI. |

### Commands

| Command | Status | Notes |
| --- | --- | --- |
| `version` | ✅ **Done** | The `version` command is implemented. |
| `extensions` | ❌ **Not Started** | Implement the `extensions` command for listing and managing extensions. |
| `mcp` | ❌ **Not Started** | Implement the `mcp` command. |

### Other Features

| Feature | Status | Notes |
| --- | --- | --- |
| **Zed Integration** | ❌ **Not Started** | Implement the integration with the Zed editor. |
| **Window Title Management** | ❌ **Not Started** | Implement setting the terminal window title. |
| **Memory Management** | ❌ **Not Started** | Implement memory management and relaunching with adjusted memory settings. |
| **Startup Warnings** | ❌ **Not Started** | Implement displaying startup warnings. |
| **Custom Themes** | ❌ **Not Started** | Implement support for custom themes. |
| **Logging** | ❌ **Not Started** | Implement logging for debugging and auditing. |
| **Kitty Keyboard Protocol** | ❌ **Not Started** | Implement support for the Kitty Keyboard Protocol. |
| **Screen Reader Support** | ❌ **Not Started** | Ensure the CLI is accessible to screen readers. |

## Testing

A comprehensive test suite will be developed alongside the features. The goal is to have a high level of test coverage to ensure the stability and correctness of the Go CLI.

Run tests with:
```bash
go test ./...
```