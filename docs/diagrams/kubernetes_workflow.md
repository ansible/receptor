# Kubernetes Worker Workflow

This document provides comprehensive diagrams documenting the Kubernetes worker implementation in Receptor. The Kubernetes worker executes work units by creating and managing Kubernetes pods, with two different streaming methods for communication.

## Table of Contents

- [Kubernetes Worker Workflow](#kubernetes-worker-workflow)
  - [Table of Contents](#table-of-contents)
  - [Purpose](#purpose)
    - [Core Capabilities](#core-capabilities)
    - [Use Cases](#use-cases)
  - [Overview](#overview)
  - [Architecture Components](#architecture-components)
    - [Core Structures](#core-structures)
    - [Key Interfaces](#key-interfaces)
  - [Workflow Diagrams](#workflow-diagrams)
    - [Diagram 1: Overall Kubernetes Worker Flow](#diagram-1-overall-kubernetes-worker-flow)
    - [Diagram 2: Authentication Flow](#diagram-2-authentication-flow)
    - [Diagram 3: Pod Creation and Lifecycle](#diagram-3-pod-creation-and-lifecycle)
    - [Diagram 4: Logger Streaming Method (Recommended)](#diagram-4-logger-streaming-method-recommended)
    - [Diagram 5: Logger Reconnection Logic](#diagram-5-logger-reconnection-logic)
    - [Diagram 6: TCP Streaming Method (Legacy)](#diagram-6-tcp-streaming-method-legacy)
    - [Diagram 7: Error Handling and Retry Logic](#diagram-7-error-handling-and-retry-logic)
  - [Key Features](#key-features)
    - [Resilience Mechanisms](#resilience-mechanisms)
    - [Configuration Flexibility](#configuration-flexibility)
  - [Configuration Options](#configuration-options)
    - [Environment Variables](#environment-variables)
    - [Worker Configuration](#worker-configuration)
  - [Work States](#work-states)
  - [Error Scenarios and Handling](#error-scenarios-and-handling)
    - [Kubernetes API Errors](#kubernetes-api-errors)
      - [Kube API Timeout](#kube-api-timeout)
      - [Kube API Connection Refused](#kube-api-connection-refused)
      - [Kube API Domain Name Cannot Be Resolved](#kube-api-domain-name-cannot-be-resolved)
      - [Kube API Returns Malformed Payload](#kube-api-returns-malformed-payload)
      - [Kube API TLS Error](#kube-api-tls-error)
      - [Kube API Is Too Old](#kube-api-is-too-old)
      - [Kube API Is Too New](#kube-api-is-too-new)
      - [Kube API Authentication Error](#kube-api-authentication-error)
    - [Pod Lifecycle Errors](#pod-lifecycle-errors)
      - [Pod Cannot Be Scheduled](#pod-cannot-be-scheduled)
      - [Pod Is Killed](#pod-is-killed)
    - [Container Execution Errors](#container-execution-errors)
      - [Container Executing Work Is Killed](#container-executing-work-is-killed)
      - [Other Containers in Pod Are Killed](#other-containers-in-pod-are-killed)
    - [Invalid Input Handling](#invalid-input-handling)
      - [Invalid PodTemplate Provided](#invalid-podtemplate-provided)
    - [Long-Running Job Scenarios](#long-running-job-scenarios)
      - [Job Running for 40+ Hours](#job-running-for-40-hours)
      - [Context Cancellation](#context-cancellation)
    - [Log and Disk I/O Errors](#log-and-disk-io-errors)
      - [Log Returned by Kube API Is Too Long](#log-returned-by-kube-api-is-too-long)
      - [Cannot Write Logs to Disk](#cannot-write-logs-to-disk)
    - [Known Unknowns](#known-unknowns)
  - [Related Documentation](#related-documentation)

## Purpose

The Kubernetes worker (`kubernetes.go`) enables Receptor to execute work units by creating and managing Kubernetes pods. This component provides a way to run containerized workloads in Kubernetes clusters.

### Core Capabilities

The Kubernetes worker provides:

1. **Creating Kubernetes pods** on demand with specified container images and commands
2. **Streaming input data** to the pod via stdin
3. **Capturing output** (stdout/stderr) from the pod via Kubernetes log streaming or TCP
4. **Managing pod lifecycle** from creation through completion, handling both success and failure scenarios

### Use Cases

The Kubernetes worker can execute any containerized workload, making it suitable for running custom scripts, batch jobs, data processing tasks, or any containerized service that accepts stdin and produces stdout/stderr.

One notable use case is **Ansible Automation Platform (AAP)**, which uses this Kubernetes worker to execute Ansible playbooks in Kubernetes clusters. AAP's Controller service submits work units to Receptor, which then uses this Kubernetes worker to execute them in pods, providing scalability, isolation, and resource management capabilities.

For more details on how work is submitted to Receptor, see:

- [Receptor Work Submit Flow](./receptor_work_submit_flow.md)

## Overview

The Kubernetes worker (`KubeUnit`) implements the `WorkUnit` interface to execute work by:

1. Creating Kubernetes pods with specified container images/commands
2. Streaming stdin data to the pod
3. Streaming stdout/stderr logs from the pod
4. Monitoring pod lifecycle and handling completion/failures

Two streaming methods are supported:

- **Logger Method**: Uses Kubernetes log streaming API for stdout/stderr (recommended for K8s >= 1.23.14)
  - **Stdin streaming**: Uses SPDY protocol via Kubernetes attach API (similar to `kubectl attach`)
  - **Stdout streaming**: Uses Kubernetes logs API with automatic reconnection support
- **TCP Method**: Pod connects back via TCP (legacy, simpler but less robust)

**What is SPDY?** SPDY is a deprecated HTTP/2 precursor protocol that Kubernetes uses for streaming operations like `kubectl exec` and `kubectl attach`. The SPDY executor creates a multiplexed connection to the Kubernetes API server that allows bidirectional streaming to/from containers. Receptor uses the Kubernetes client-go library's `remotecommand.NewSPDYExecutor()` to create these connections for streaming stdin to pods.

## Architecture Components

### Core Structures

- **KubeUnit**: Main work unit implementation for Kubernetes
- **KubeExtraData**: Stores pod configuration (image, command, namespace, etc.)
- **KubeAPIWrapper**: Wrapper around Kubernetes client-go for testability
- **KubeWorkerCfg**: Configuration object for command-line setup

### Key Interfaces

- **KubeAPIer**: Interface for Kubernetes API operations (allows mocking for tests)
- **WorkUnit**: Base interface that KubeUnit implements

## Workflow Diagrams

### Diagram 1: Overall Kubernetes Worker Flow

This diagram shows the complete flow from work submission to completion using the logger streaming method.

```mermaid
sequenceDiagram
    participant Client
    participant Workceptor as Work Service<br/>(Workceptor)
    participant KubeUnit as KubeUnit
    participant KubeAPI as Kubernetes API
    participant Pod
    participant Logger as Log Stream
    participant StdoutFile as stdout file

    Note over Client, StdoutFile: Work submission with Kubernetes worker

    Client->>Workceptor: Submit work (worktype: kubernetes)
    Workceptor->>KubeUnit: Create KubeUnit instance
    Workceptor->>KubeUnit: Start()

    Note over KubeUnit: UpdateBasicStatus(WorkStatePending,<br/>"Connecting to Kubernetes")
    
    KubeUnit->>KubeUnit: connectToKube()
    alt AuthMethod: kubeconfig
        KubeUnit->>KubeUnit: connectUsingKubeconfig()
        Note over KubeUnit: Load kubeconfig file<br/>or use default
    else AuthMethod: incluster
        KubeUnit->>KubeUnit: connectUsingIncluster()
        Note over KubeUnit: Use in-cluster config
    else AuthMethod: runtime
        KubeUnit->>KubeUnit: connectUsingKubeconfig()
        Note over KubeUnit: Use runtime-provided config
    end
    
    KubeUnit->>KubeAPI: Create clientset
    KubeAPI-->>KubeUnit: Kubernetes clientset

    KubeUnit->>KubeUnit: RunWorkUsingLogger()

    alt Pod exists (resume)
        KubeUnit->>KubeAPI: Get pod by name
        KubeAPI-->>KubeUnit: Existing pod
        Note over KubeUnit: skipStdin = true
    else New pod
        KubeUnit->>KubeUnit: CreatePod()
        KubeUnit->>KubeAPI: Create pod manifest
        KubeAPI-->>KubeUnit: Pod created
        
        Note over KubeUnit: Wait for pod Ready state
        
        KubeUnit->>KubeAPI: Watch pod events
        KubeAPI-->>KubeUnit: Pod status updates
        
        alt Pod Ready
            KubeAPI-->>KubeUnit: Pod Running + Ready
        else Pod Failed
            KubeAPI-->>KubeUnit: Pod Failed
            KubeUnit->>KubeUnit: Mark as Failed
        else Pod Completed
            KubeAPI-->>KubeUnit: Pod Succeeded
            KubeUnit->>KubeUnit: Handle completion
        end
        
        Note over KubeUnit: skipStdin = false
    end

    alt !skipStdin (new pod)
        KubeUnit->>KubeAPI: Create SPDY executor (attach)
        KubeAPI-->>KubeUnit: Executor ready
        
        KubeUnit->>KubeUnit: Wait for container Running state
        loop Retry until Running
            KubeUnit->>KubeAPI: Get pod status
            KubeAPI-->>KubeUnit: Container state
            alt Container Waiting
                Note over KubeUnit: Retry with Fibonacci backoff
            else Container Terminated
                KubeUnit->>KubeUnit: Mark as Failed
            end
        end
        
        par Stream stdin
            KubeUnit->>KubeAPI: Stream stdin via SPDY
            KubeAPI->>Pod: Forward stdin data
            Pod-->>KubeAPI: EOF received
            KubeAPI-->>KubeUnit: Stream complete
            KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateRunning)
        and Stream stdout
            KubeUnit->>KubeAPI: Get log stream
            KubeAPI->>Logger: Open log stream (with timestamps)
            Logger->>KubeUnit: Stream log lines
            
            loop Process log lines
                Logger->>KubeUnit: Read log line
                KubeUnit->>KubeUnit: ProcessLogLine()<br/>(strip timestamp,<br/>detect duplicates)
                KubeUnit->>StdoutFile: Write processed line
            end
            
            alt EOF received
                KubeUnit->>KubeAPI: Get pod status
                alt Container Terminated
                    Logger->>KubeUnit: EOF (expected)
                    KubeUnit->>KubeUnit: Check exit code and termination reason
                    alt Exit code 0
                        KubeUnit->>KubeUnit: Mark as Succeeded
                    else Exit code != 0 AND reason is "Completed"/"Error"
                        KubeUnit->>KubeUnit: Mark as Succeeded<br/>(normal completion with error)
                    else Exit code != 0 AND reason is "OOMKilled"/"Evicted"/etc
                        KubeUnit->>KubeUnit: Mark as Failed<br/>(execution interrupted)
                    end
                else Container Running
                    Logger->>KubeUnit: EOF (unexpected - may be 4hr timeout)
                    KubeUnit->>KubeUnit: Reconnect logic
                    Note over KubeUnit: KubeLoggingWithReconnect()
                end
            end
        end
    else skipStdin (resume)
        KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateRunning)
        KubeUnit->>KubeAPI: Get log stream (resume from sinceTime)
        
        loop Stream logs
            KubeUnit->>KubeAPI: Read log lines
            KubeAPI->>KubeUnit: Log lines with timestamps
            KubeUnit->>KubeUnit: ProcessLogLine()<br/>(skip duplicates)
            KubeUnit->>StdoutFile: Write new lines
        end
    end

    KubeUnit->>Workceptor: WorkStateSucceeded or WorkStateFailed
    Workceptor-->>Client: Work completion status
```

### Diagram 2: Authentication Flow

This diagram details the three authentication methods supported by the Kubernetes worker.

```mermaid
flowchart TD
    Start([KubeUnit.Start]) --> Connect[connectToKube]
    Connect --> AuthMethod{Check authMethod}
    
    AuthMethod -->|kubeconfig| Kubeconfig[connectUsingKubeconfig]
    AuthMethod -->|incluster| InCluster[connectUsingIncluster]
    AuthMethod -->|runtime| Runtime[connectUsingKubeconfig<br/>with runtime config]
    
    Kubeconfig --> CheckConfig{KubeConfig<br/>provided?}
    CheckConfig -->|No| DefaultConfig[Load default kubeconfig<br/>~/.kube/config]
    CheckConfig -->|Yes| LoadConfig[Read KubeConfig<br/>from file/param]
    
    DefaultConfig --> ParseDefault[Parse config<br/>get namespace]
    LoadConfig --> ParseCustom[Parse config bytes<br/>get namespace]
    
    ParseDefault --> BuildDefault[BuildConfigFromFlags<br/>masterURL, kubeconfig]
    ParseCustom --> BuildCustom[ClientConfigFromBytes<br/>then ClientConfig]
    
    InCluster --> ClusterConfig[rest.InClusterConfig<br/>Read from:<br/>/var/run/secrets/kubernetes.io/serviceaccount]
    
    BuildDefault --> CheckNamespace{Namespace<br/>provided?}
    BuildCustom --> CheckNamespace
    ClusterConfig --> CheckNamespace
    
    CheckNamespace -->|No| GetNamespace[Get namespace from<br/>kubeconfig context]
    CheckNamespace -->|Yes| SetNamespace[Use provided namespace]
    
    GetNamespace --> SetConfigVars
    SetNamespace --> SetConfigVars[Set config QPS/Burst<br/>Set rate limiter]
    
    SetConfigVars --> CreateClientset[kubernetes.NewForConfig<br/>Create clientset]
    CreateClientset --> Done([Authentication Complete])
    
    Runtime --> RuntimeCheck{Runtime config<br/>provided?}
    RuntimeCheck -->|No| RuntimeError[Error: secret_kube_config<br/>must be provided]
    RuntimeCheck -->|Yes| LoadRuntime[Load config from params]
    LoadRuntime --> BuildCustom
```

### Diagram 3: Pod Creation and Lifecycle

This diagram shows how pods are created and how the system waits for them to become ready.

```mermaid
sequenceDiagram
    participant KubeUnit
    participant KubeAPI as Kubernetes API
    participant Pod
    participant Watch as Watch Interface

    Note over KubeUnit, Watch: Pod creation and readiness check

    KubeUnit->>KubeUnit: CreatePod()

    alt Custom Pod Spec (KubePod provided)
        KubeUnit->>KubeUnit: Decode YAML/JSON pod spec
        KubeUnit->>KubeUnit: Validate "worker" container exists
        KubeUnit->>KubeUnit: Set Stdin=true, StdinOnce=true
        KubeUnit->>KubeUnit: Set RestartPolicy=Never
        KubeUnit->>KubeUnit: Set GenerateName from namePrefix
    else Simple Pod Spec
        KubeUnit->>KubeUnit: Create PodSpec with:<br/>- Image from config<br/>- Command from config<br/>- Args from params<br/>- Container name: "worker"<br/>- Stdin=true, StdinOnce=true<br/>- RestartPolicy=Never
    end

    KubeUnit->>KubeUnit: Add environment variables<br/>(if provided)
    KubeUnit->>KubeAPI: Create pod
    KubeAPI->>Pod: Schedule pod
    KubeAPI-->>KubeUnit: Pod created (with generated name)

    KubeUnit->>KubeUnit: UpdateFullStatus(WorkStatePending,<br/>"Pod created")
    KubeUnit->>KubeUnit: Store pod.Name in ExtraData

    Note over KubeUnit, Watch: Wait for pod to be running and ready

    KubeUnit->>KubeAPI: Create ListWatch with fieldSelector
    KubeAPI-->>KubeUnit: ListWatch interface

    alt Timeout configured
        KubeUnit->>KubeUnit: Create context with<br/>podPendingTimeout
    else No timeout
        KubeUnit->>KubeUnit: Use parent context
    end

    KubeUnit->>KubeUnit: Sleep 2 seconds
    KubeUnit->>Watch: UntilWithSync(condition: podRunningAndReady)

    Watch->>Pod: Watch pod events

    loop Watch events
        Pod->>Watch: Pod event (Added/Modified)
        Watch->>KubeUnit: Event received
        
        KubeUnit->>KubeUnit: podRunningAndReady() check
        
        alt Pod Phase: Running or Pending
            KubeUnit->>KubeUnit: Check PodReady condition
            alt PodReady == True
                Watch-->>KubeUnit: Pod ready event
            else ContainersReady == False
                KubeUnit->>KubeUnit: Check container status
                alt ImagePullBackOff
                    KubeUnit->>KubeUnit: Retry check (max 3 times)
                    alt Retries exhausted
                        Watch-->>KubeUnit: Error: ErrImagePullBackOff
                    end
                end
            end
        else Pod Phase: Failed
            Watch-->>KubeUnit: Error: ErrPodFailed
        else Pod Phase: Succeeded
            Watch-->>KubeUnit: Error: ErrPodCompleted
        else Pod Deleted (during startup)
            Watch-->>KubeUnit: Error: NotFound
        end
    end

    alt Pod Ready
        KubeUnit->>KubeUnit: Store pod object
        KubeUnit-->>KubeUnit: CreatePod() returns nil
    else Error
        KubeUnit->>KubeUnit: Handle error
        alt ErrPodCompleted
            KubeUnit->>KubeUnit: Check exit code
            alt Exit code != 0
                KubeUnit-->>KubeUnit: Return container failure error
            else Exit code == 0
                KubeUnit-->>KubeUnit: Return ErrPodCompleted
            end
        else Other error
            KubeUnit->>KubeAPI: Try to get pod logs (fallback)
            KubeUnit-->>KubeUnit: Return error with details
        end
    end
```

### Diagram 4: Logger Streaming Method (Recommended)

This diagram shows the logger streaming method flow, including stdin streaming and log retrieval with reconnection support.

```mermaid
sequenceDiagram
    participant KubeUnit
    participant KubeAPI as Kubernetes API
    participant Pod
    participant LogStream as Log Stream
    participant StdoutFile as stdout file
    participant StdinFile as stdin file

    Note over KubeUnit, StdinFile: Logger streaming method workflow

    KubeUnit->>KubeUnit: RunWorkUsingLogger()

    alt Resume existing pod
        KubeUnit->>KubeAPI: Get pod by name
        KubeAPI-->>KubeUnit: Existing pod
        Note over KubeUnit: skipStdin = true<br/>(stdin already sent in initial run,<br/>only need to reconnect to stdout logs)
    else Create new pod
        KubeUnit->>KubeAPI: CreatePod()
        KubeAPI-->>KubeUnit: Pod created
        Note over KubeUnit: skipStdin = false<br/>(must send stdin to new pod)
    end

    Note over KubeUnit: streamWait.Add(2) - always expects 2 completions

    alt !skipStdin (new pod)
        Note over KubeUnit: Will launch 2 goroutines (stdin + stdout)
        Note over KubeUnit: Wait for container Running state
        
        loop Check container state
            KubeUnit->>KubeAPI: Get pod status
            KubeAPI-->>KubeUnit: Container status
            alt Container Waiting
                KubeUnit->>KubeUnit: Sleep with Fibonacci backoff
                KubeUnit->>KubeAPI: Retry get pod
            else Container Terminated
                KubeUnit->>KubeUnit: Mark as Failed
            else Container Running
                Note over KubeUnit: Break loop
            end
        end

        KubeUnit->>KubeAPI: Create SPDY executor<br/>(SubResource attach)
        KubeAPI-->>KubeUnit: Executor ready

        par Stream stdin to pod (goroutine 1)
            KubeUnit->>StdinFile: Open stdin file
            StdinFile-->>KubeUnit: File reader
            
            KubeUnit->>KubeAPI: StreamWithContext(executor,<br/>StreamOptions{Stdin: reader})
            
            loop Stream stdin data
                StdinFile->>KubeUnit: Read chunk
                KubeUnit->>KubeAPI: Send chunk via SPDY
                KubeAPI->>Pod: Forward to container stdin
            end
            
            StdinFile->>KubeUnit: EOF
            KubeUnit->>KubeAPI: Close stdin stream
            KubeAPI->>Pod: Send EOF
            Pod-->>KubeAPI: Stdin closed
            KubeAPI-->>KubeUnit: Stream complete
            
            alt Stdin stream success
                KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateRunning)
            else Stdin stream error
                KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateFailed)
            end
        and Stream stdout from pod (goroutine 2)
            KubeUnit->>StdoutFile: Open stdout file
            StdoutFile-->>KubeUnit: File writer
            
            alt Reconnect supported (K8s >= 1.23.14)
                KubeUnit->>KubeUnit: KubeLoggingWithReconnect()
                Note over KubeUnit: See Diagram 5 for details
            else No reconnect (K8s < 1.23.14)
                KubeUnit->>KubeUnit: kubeLoggingNoReconnect()
                KubeUnit->>KubeAPI: GetLogs(Follow=true,<br/>Timestamps=false)
                KubeAPI->>LogStream: Open log stream
                LogStream-->>KubeUnit: Stream handle
                
                KubeUnit->>LogStream: io.Copy(stdout, stream)
                loop Read log stream
                    LogStream->>KubeUnit: Log line
                    KubeUnit->>StdoutFile: Write line
                end
                
                alt Stream error
                    LogStream-->>KubeUnit: Error (EOF or other)
                    Note over KubeUnit: Log stream terminated<br/>(may be due to rotation or 4hr timeout)
                end
            end
        end
    else skipStdin (resume)
        Note over KubeUnit: Will launch 1 goroutine (stdout only)<br/>streamWait.Done() called immediately<br/>(no stdin goroutine needed)

        KubeUnit->>KubeUnit: streamWait.Done()<br/>(count stdin as "complete")
        KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateRunning)

        Note over KubeUnit: Launch stdout goroutine
        KubeUnit->>KubeAPI: GetLogs(Follow=true,<br/>Timestamps=true,<br/>SinceTime=lastTimestamp)

        loop Stream logs from sinceTime
            KubeUnit->>KubeAPI: Read log lines
            KubeAPI->>LogStream: Stream lines with timestamps
            LogStream->>KubeUnit: Log line
            KubeUnit->>KubeUnit: ProcessLogLine()<br/>(skip duplicates)
            KubeUnit->>StdoutFile: Write new lines only
        end
    end

    Note over KubeUnit: streamWait.Wait() - blocks until 2 completions<br/>(New pod: 2 goroutines | Resume: 1 goroutine + 1 immediate Done())
    
    alt Both streams successful
        KubeUnit->>KubeUnit: UpdateFullStatus(WorkStateSucceeded)
    else Error occurred
        KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateFailed)
    end
```

### Diagram 5: Logger Reconnection Logic

This diagram details the sophisticated reconnection logic used in `KubeLoggingWithReconnect()` to handle log stream disconnections.

```mermaid
flowchart TD
    Start([KubeLoggingWithReconnect]) --> MainLoop[Main reconnection loop]
    
    MainLoop --> CheckStdin{stdinErr<br/>!= nil?}
    CheckStdin -->|Yes| Exit1([Exit - stdin failed])
    CheckStdin -->|No| GetPod[Get pod with Fibonacci<br/>retry backoff]
    
    GetPod --> PodError{Pod get<br/>error?}
    PodError -->|Yes| RetryPod{Retries<br/>remaining?}
    RetryPod -->|Yes| SleepPod[Sleep with<br/>Fibonacci backoff]
    SleepPod --> GetPod
    RetryPod -->|No| Exit2([Exit - pod error])
    
    PodError -->|No| ResetSuccess[Reset successfulWrite flag]
    ResetSuccess --> GetLogs[Get log stream<br/>with timestamps<br/>SinceTime sinceTime]
    
    GetLogs --> LogError{Log stream<br/>error?}
    LogError -->|Yes| Exit3([Exit - log stream error])
    LogError -->|No| ReadLoop[Read loop: process log lines]
    
    ReadLoop --> ReadLine[ReadString '\n']
    ReadLine --> ReadError{Read<br/>error?}
    
    ReadError -->|No| ProcessLine[ProcessLogLine:<br/>1. Parse timestamp<br/>2. Strip timestamp<br/>3. Check for duplicates<br/>4. Update sinceTime]
    ProcessLine --> CheckSkip{shouldSkip<br/>= true?}
    CheckSkip -->|Yes| ReadLoop
    CheckSkip -->|No| WriteStdout[Write to stdout file]
    WriteStdout --> SetSuccess[Set successfulWrite = true<br/>Reset retry counter]
    SetSuccess --> ReadLoop
    
    ReadError -->|Yes| CheckCancel{Context<br/>Canceled?}
    CheckCancel -->|Yes| CheckState{State !=<br/>Succeeded/Failed?}
    CheckState -->|Yes| Exit4([Exit - mark as Failed])
    CheckState -->|No| Exit5([Exit - already complete])
    
    CheckCancel -->|No| CheckEOF{Error ==<br/>EOF?}
    CheckEOF -->|No| RetryRead{Retries<br/>remaining?}
    RetryRead -->|Yes| SleepRead[Sleep with<br/>Fibonacci backoff]
    SleepRead --> MainLoop
    RetryRead -->|No| Exit6([Exit - non-EOF error])
    
    CheckEOF -->|Yes| GetPodAfterEOF[Get pod status<br/>after EOF]
    GetPodAfterEOF --> PodErrEOF{Pod get<br/>error?}
    PodErrEOF -->|Yes| MainLoop
    PodErrEOF -->|No| CheckContainer{Found worker<br/>container?}
    
    CheckContainer -->|No| Exit7([Exit - container not found])
    CheckContainer -->|Yes| CheckState2{Container<br/>state?}
    
    CheckState2 -->|Running| SleepEOF[Sleep with Fibonacci backoff<br/>Possible 4hr timeout or<br/>transition to terminated<br/>No retry limit - continues until<br/>terminated or context canceled]
    SleepEOF --> MainLoop
    
    CheckState2 -->|Terminated| CheckExitCode{Exit<br/>code?}
    CheckExitCode -->|0| LogSuccess[Mark as Succeeded<br/>Log: Completed successfully]
    CheckExitCode -->|!= 0| CheckReason{Terminated<br/>reason?}
    
    CheckReason -->|Completed/Error| LogErrorComplete[Mark as Succeeded<br/>Log: Completed with error<br/>exit code<br/>Normal completion]
    CheckReason -->|OOMKilled/Evicted/etc| LogInterrupted[Mark as Failed<br/>Set stdoutErr<br/>Log: Execution interrupted]
    
    LogSuccess --> CheckLastLine{Last line<br/>has data?}
    LogErrorComplete --> CheckLastLine
    LogInterrupted --> Exit9([Exit - interrupted])
    
    CheckLastLine -->|Yes| ProcessLast[ProcessLogLine for last line]
    ProcessLast --> WriteLast[Write last line to stdout]
    WriteLast --> Exit10([Exit - normal completion])
    CheckLastLine -->|No| Exit10

    CheckState2 -->|Unknown| LogUnknown[Debug log: Will continue<br/>Misleading: actually fails immediately<br/>Mark as Failed + set stdoutErr]
    LogUnknown --> Exit11([Exit - job failed])
```

### Diagram 6: TCP Streaming Method (Legacy)

This diagram shows the TCP streaming method where the pod connects back to the host.

```mermaid
sequenceDiagram
    participant KubeUnit
    participant TCPListener as TCP Listener
    participant KubeAPI as Kubernetes API
    participant Pod
    participant TCPConn as TCP Connection
    participant StdinFile as stdin file
    participant StdoutFile as stdout file

    Note over KubeUnit, StdoutFile: TCP streaming method workflow

    KubeUnit->>KubeUnit: runWorkUsingTCP()

    Note over KubeUnit: Step 1: Create TCP listener

    KubeUnit->>KubeUnit: getDefaultInterface()<br/>Find first non-loopback interface
    KubeUnit->>TCPListener: Listen on interface IP:0<br/>(auto-assign port)
    TCPListener-->>KubeUnit: Listener ready

    KubeUnit->>KubeUnit: Split address to get host and port

    par Accept connection (async)
        TCPListener->>TCPListener: Accept() - wait for connection
    and Create pod with env vars
        KubeUnit->>KubeUnit: CreatePod() with env:<br/>RECEPTOR_HOST={host}<br/>RECEPTOR_PORT={port}
        
        KubeUnit->>KubeAPI: Create pod manifest
        KubeAPI->>Pod: Schedule and start pod
        
        Pod->>Pod: Read RECEPTOR_HOST and<br/>RECEPTOR_PORT env vars
        
        Pod->>TCPConn: Dial TCP connection<br/>to {host}:{port}
        TCPConn->>TCPListener: Connection established
        TCPListener-->>TCPConn: Connection accepted
    end

    TCPListener->>KubeUnit: Connection received
    KubeUnit->>TCPListener: Close listener (only accept one)
    TCPListener->>TCPConn: TCP connection object

    Note over KubeUnit: Step 2: Stream stdin to pod

    KubeUnit->>StdinFile: Open stdin file
    StdinFile-->>KubeUnit: File reader

    KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStatePending,<br/>"Sending stdin to pod")

    par Write stdin
        loop Stream stdin data
            StdinFile->>KubeUnit: Read chunk
            KubeUnit->>TCPConn: Write chunk
            TCPConn->>Pod: Forward data
        end
        
        StdinFile->>KubeUnit: EOF
        KubeUnit->>TCPConn: CloseWrite() - send FIN
        TCPConn->>Pod: EOF signal
    and Monitor stdin completion
        KubeUnit->>KubeUnit: Wait for stdin.Done()
        alt EOF success
            KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateRunning)
        else stdin error
            KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateFailed)
        end
    end

    Note over KubeUnit: Step 3: Read stdout from pod

    KubeUnit->>StdoutFile: Open stdout file
    StdoutFile-->>KubeUnit: File writer

    loop Read stdout data
        TCPConn->>KubeUnit: Read chunk
        KubeUnit->>StdoutFile: Write chunk
    end

    TCPConn->>KubeUnit: EOF (connection closed)
    
    alt Context not canceled
        KubeUnit->>KubeUnit: UpdateBasicStatus(WorkStateSucceeded,<br/>"Finished")
    end

    Note over KubeUnit: TCP method limitations:<br/>- No automatic reconnection<br/>- Simpler but less robust<br/>- Requires pod to connect back
```

### Diagram 7: Error Handling and Retry Logic

This diagram shows the comprehensive error handling and retry mechanisms throughout the Kubernetes worker.

```mermaid
flowchart TD
    Start([Error Handling Overview]) --> ErrorTypes[Error Categories]
    
    ErrorTypes --> PodErrors[Pod Lifecycle Errors]
    ErrorTypes --> StreamErrors[Stream Errors]
    ErrorTypes --> AuthErrors[Authentication Errors]
    ErrorTypes --> TimeoutErrors[Timeout Errors]
    
    PodErrors --> PodFailed[ErrPodFailed:<br/>Pod Phase = Failed]
    PodErrors --> PodCompleted[ErrPodCompleted:<br/>Pod Phase = Succeeded]
    PodErrors --> ImagePullBack[ErrImagePullBackOff:<br/>Container waiting - ImagePullBackOff]
    PodErrors --> NotFound[Pod NotFound:<br/>Pod deleted during startup<br/>or doesn't exist]

    StreamErrors --> SPDYCreationError[SPDY Executor Creation Error:<br/>Cannot create executor]
    StreamErrors --> StdinStreamError[Stdin Streaming Error:<br/>Error streaming to pod]
    StreamErrors --> StdoutError[Stdout Stream Error:<br/>Log stream EOF/timeout]
    StreamErrors --> NonEOFError[Non-EOF Error:<br/>Unexpected stream error]

    AuthErrors --> ConfigError[Config Parse Error:<br/>Invalid kubeconfig]
    AuthErrors --> ClusterError[InCluster Error:<br/>Not running in cluster]

    TimeoutErrors --> PodPendingTimeout[Pod Pending Timeout:<br/>Pod didn't become ready]

    PodFailed --> HandlePodFailed[Handle: Return error with details]

    PodCompleted --> HandlePodCompleted[Handle:<br/>1. Check container exit code<br/>2. If exit != 0, return error<br/>3. If exit == 0, return ErrPodCompleted]

    ImagePullBack --> HandleImagePull[Handle:<br/>1. Retry check 3 times<br/>2. If still failing, return ErrImagePullBackOff]

    NotFound --> HandleNotFound[Handle: Return error with details]

    SPDYCreationError --> FailSPDY[Handle:<br/>Mark work as Failed immediately<br/>No retries]

    StdinStreamError --> RetryStdin{Retries<br/>remaining?}
    RetryStdin -->|Yes| RetryStdinAction[Retry StreamWithContext<br/>200ms delay between retries<br/>Max: GetKubeRetryCount times]
    RetryStdinAction --> RetryStdin
    RetryStdin -->|No| FailStdin[Mark work as Failed<br/>Signal stdout to stop]
    
    StdoutError --> CheckContext{Context<br/>Canceled?}
    CheckContext -->|Yes| CheckState{State !=<br/>Succeeded/Failed?}
    CheckState -->|Yes| FailContext[Mark as Failed]
    CheckState -->|No| ExitContext[Already complete - exit]
    
    CheckContext -->|No| CheckEOF{Error ==<br/>EOF?}
    CheckEOF -->|Yes| CheckPodState[Get pod state]
    CheckPodState --> PodRunning{Container<br/>Running?}
    PodRunning -->|Yes| ReconnectLogs[Reconnect log stream<br/>with Fibonacci backoff<br/>Continue indefinitely<br/>No retry limit - continues until<br/>container terminates or context canceled]
    
    PodRunning -->|No| PodTerminated{Container<br/>Terminated?}
    PodTerminated -->|Yes| CheckExit{Exit<br/>code == 0?}
    CheckExit -->|Yes| Success[Mark as Succeeded]
    CheckExit -->|No| CheckReason{Terminated<br/>reason?}
    CheckReason -->|Completed/Error| SuccessNormal[Mark as Succeeded<br/>Normal completion with error]
    CheckReason -->|OOMKilled/etc| FailInterrupted[Mark as Failed<br/>Interrupted execution]
    
    CheckEOF -->|No| RetryNonEOF{Retries<br/>remaining?}
    RetryNonEOF -->|Yes| RetryLogRead[Retry read with Fibonacci backoff]
    RetryLogRead --> ReconnectLogs
    RetryNonEOF -->|No| FailNonEOF[Mark as Failed]
    
    NonEOFError --> RetryNonEOF
    
    ConfigError --> FailAuth[Mark as Failed:<br/>Cannot authenticate]
    ClusterError --> FailAuth
    
    PodPendingTimeout --> FailTimeout[Mark as Failed:<br/>Pod didn't become ready]
```

## Key Features

### Resilience Mechanisms

1. **Automatic Reconnection**: Logger method automatically reconnects on stream disconnection
2. **Retry Logic**: Fibonacci backoff for transient errors, with no retry limit for EOF with Running state
3. **Duplicate Detection**: Timestamp-based log line deduplication during reconnections
4. **Timeout Handling**: Configurable timeouts for pod pending state
5. **Graceful Degradation**: Falls back to no-reconnect method for older Kubernetes versions
6. **Long-Running Job Support**: Continues attempting reconnection indefinitely when EOF occurs with Running containers (handles 4-hour log stream timeouts)

### Configuration Flexibility

1. **Multiple Auth Methods**: kubeconfig, incluster, or runtime
2. **Runtime Overrides**: Allow dynamic image/command/params/pod specification
3. **Stream Method Selection**: Choose logger or TCP method
4. **Rate Limiting**: Configurable QPS and burst for API calls
5. **Timeout Configuration**: Environment variables for timeouts and retry counts

## Configuration Options

### Environment Variables

- `RECEPTOR_KUBE_TIMEOUT_START`: Base timeout for retries (default: 1s, max: 1m)
- `RECEPTOR_KUBE_RETRY_COUNT`: Number of retries (default: 5, max: 100)
- `RECEPTOR_KUBE_SUPPORT_RECONNECT`: Enable/disable/auto reconnect (default: enabled)
- `RECEPTOR_KUBE_CLIENTSET_QPS`: API rate limit QPS (default: 100)
- `RECEPTOR_KUBE_CLIENTSET_BURST`: API rate limit burst (default: 10x QPS)
- `RECEPTOR_KUBE_CLIENTSET_RATE_LIMITER`: Rate limiter type (never/always/tokenbucket)

### Worker Configuration

```yaml
- work-kubernetes:
    workType: k8s-worker
    namespace: default
    image: my-image:latest
    command: /bin/sh
    params: -c
    authMethod: incluster
    streamMethod: logger
    allowRuntimeCommand: false
    allowRuntimeParams: true
    deletePodOnRestart: true
```

## Work States

1. **WorkStatePending (0)**: Initial state, connecting to Kubernetes or creating pod
2. **WorkStateRunning (1)**: Pod running, streaming data
3. **WorkStateSucceeded (2)**: Work completed successfully. Determined by:
   - Exit code 0, OR
   - Exit code != 0 but termination reason is "Completed" or "Error" (indicates normal program completion with error)
4. **WorkStateFailed (3)**: Work failed. Determined by:
   - Exit code != 0 AND termination reason indicates interruption (OOMKilled, Evicted, etc.), OR
   - Other errors occurred during execution (stream errors, pod failures, etc.)
5. **WorkStateCanceled (4)**: Work was canceled, pod deleted

## Error Scenarios and Handling

This section documents how the Kubernetes worker handles various error conditions. Understanding these scenarios is critical for debugging production issues and ensuring reliable job execution.

### Kubernetes API Errors

#### Kube API Timeout

**What happens:**

- Timeouts occur during API calls (Get, Create, Watch, GetLogs)
- The underlying `client-go` library uses the context timeout if provided
- If `podPendingTimeout` is configured, pod readiness checks will timeout
- Watch operations timeout when the context is canceled

**Current handling:**

- ✅ **Pod creation**: Uses `context.WithTimeout()` if `podPendingTimeout` is set. Returns error if timeout exceeded
- ✅ **Pod readiness wait**: `UntilWithSync()` respects context timeout. Returns timeout error
- ⚠️ **API calls without explicit timeout**: Relies on context cancellation or underlying HTTP client timeouts
- ⚠️ **Retry logic**: Retries use Fibonacci backoff but may continue indefinitely if context isn't canceled

**Impact:** Jobs will fail with timeout error. Pod may remain in pending state.

#### Kube API Connection Refused

**What happens:**

- Cannot connect to Kubernetes API server (server down, network issues, firewall)

**Current handling:**

- ❌ **No explicit retry**: Connection errors during `connectToKube()` are returned immediately
- ❌ **No retry in CreatePod()**: TODO comment mentions adding retry logic but not implemented
- ✅ **Retry in log stream**: `kubeLoggingConnectionHandler()` retries up to `GetKubeRetryCount()` times with simple delay (not Fibonacci)
- ✅ **Retry in Get pod**: When resuming, retries 5 times with 200ms delay
- ✅ **Retry in log reconnection**: Main loop retries getting pod with Fibonacci backoff

**Impact:** Initial connection failure causes immediate job failure. However, transient connection issues during execution are retried.

#### Kube API Domain Name Cannot Be Resolved

**What happens:**

- DNS resolution fails for Kubernetes API server hostname

**Current handling:**

- ❌ **No explicit handling**: Treated same as connection refused
- ⚠️ **Error propagation**: Error from `BuildConfigFromFlags()` or `InClusterConfig()` bubbles up to `connectToKube()` which returns error immediately

**Impact:** Job fails at startup with DNS resolution error.

#### Kube API Returns Malformed Payload

**What happens:**

- API server returns invalid JSON/YAML or unexpected response structure

**Current handling:**

- ⚠️ **Partial handling**: Pod YAML/JSON decoding errors are caught in `CreatePod()` and returned
- ❌ **Watch/list responses**: No explicit validation of malformed API responses
- ❌ **Log stream responses**: No validation of log stream format
- ⚠️ **Error propagation**: Depends on `client-go` library to handle malformed responses

**Impact:** Likely to cause panics or unexpected behavior. Partial protection for pod spec decoding.

#### Kube API TLS Error

**What happens:**

- TLS handshake fails, certificate validation errors, certificate expired

**Current handling:**

- ❌ **No explicit handling**: TLS errors from `client-go` are propagated as-is
- ⚠️ **Error location**: TLS errors occur during `NewForConfig()` or API calls
- ⚠️ **Certificate validation**: Handled by `client-go` based on `rest.Config` TLS settings

**Impact:** Job fails with TLS error. No retry logic for TLS errors.

#### Kube API Is Too Old

**What happens:**

- Kubernetes version doesn't support required features (e.g., log stream timestamps, certain API versions)

**Current handling:**

- ✅ **Version detection**: `ShouldUseReconnect()` checks server version via `Discovery().ServerVersion()`
- ✅ **Graceful degradation**: Falls back to no-reconnect logging method for older versions
- ✅ **Compatibility check**: `IsCompatibleK8S()` validates version >= 1.23.14 for reconnect support
- ⚠️ **Feature detection**: Only checks for reconnect support. Other version-dependent features not explicitly checked.

**Impact:** Automatically falls back to legacy method. Should work on older versions.

#### Kube API Is Too New

**What happens:**

- Kubernetes version introduces breaking changes or new required fields

**Current handling:**

- ⚠️ **Limited handling**: `client-go` version compatibility should handle most cases
- ❌ **No explicit new version detection**: Assumes `client-go` handles newer API versions
- ⚠️ **API version negotiation**: Handled automatically by `client-go`

**Impact:** May work if `client-go` supports it, or may fail with API errors.

#### Kube API Authentication Error

**What happens:**

- Invalid kubeconfig, expired tokens, insufficient permissions, wrong namespace

**Current handling:**

- ✅ **kubeconfig errors**: Caught in `connectUsingKubeconfig()`, errors returned immediately
- ✅ **in-cluster errors**: `InClusterConfig()` errors caught
- ❌ **No retry**: Authentication errors are not retried (correctly, as they won't resolve)
- ⚠️ **Permission errors**: API calls return `apierrors.IsForbidden()` which propagates through error handling
- ⚠️ **Token expiration**: No token refresh logic; tokens from kubeconfig expected to be valid

**Impact:** Job fails immediately with authentication error. No automatic token refresh.

### Pod Lifecycle Errors

#### Pod Cannot Be Scheduled

**What happens:**

- No nodes available, resource constraints, node selectors/affinity rules prevent scheduling

**Current handling:**

- ✅ **Watch detects**: `podRunningAndReady()` watches for pod phase changes
- ⚠️ **Timeout handling**: If `podPendingTimeout` is set, pending pods timeout
- ⚠️ **No explicit unscheduled detection**: Doesn't specifically check `pod.Status.Conditions` for `PodScheduled: False`
- ⚠️ **Error message**: Returns generic error from `UntilWithSync()` timeout

**Impact:** Job fails with timeout if pod never schedules. Error message may not clearly indicate scheduling issue.

#### Pod Is Killed

**What happens:**

- Pod evicted due to node pressure, node shutdown, manual pod deletion via kubectl

**Current handling:**

- ✅ **Watch detects deletion during startup**: `podRunningAndReady()` watches for pod events and returns `NotFound` error if `watch.Deleted` event received while waiting for pod to become ready
- ✅ **Watch detects pod phase failures**: Returns `ErrPodFailed` if pod enters `PodFailed` phase, `ErrPodCompleted` if pod enters `PodSucceeded` phase before ready
- ⚠️ **Deletion during execution handled indirectly**: When pod is deleted during job execution:
  1. Log stream closes (EOF received)
  2. Subsequent `Get()` calls to retrieve pod status return errors (likely `NotFound`)
  3. After retries are exhausted (default 5 retries at kubernetes.go:384-406), the job fails with error "Error getting pod X/Y. Error: pods 'X' not found"
  4. No explicit `IsNotFound()` check to distinguish pod deletion from other API errors
- ⚠️ **No explicit eviction detection**: Does not check pod conditions or events for eviction-specific signals (e.g., `pod.Status.Reason == "Evicted"`)
- ⚠️ **Generic error handling**: Pod-level failures (eviction, node shutdown) are detected through watch phase changes or API Get() errors, not through specific eviction events

**Impact:** Pod deletion/eviction is detected but reported as generic API errors ("Error getting pod"). Error messages may not clearly indicate whether the pod was deleted, evicted, or experienced another failure. Handling is indirect, relies on watch events (during startup) or API errors (during execution) rather than explicit pod condition checks.

### Container Execution Errors

#### Container Executing Work Is Killed

**What happens:**

- Container running the work is killed or terminates abnormally:
  - **OOMKilled**: Container exceeded memory limit (container-level event, not pod-level)
  - **SIGKILL/SIGTERM**: Container killed by runtime or scheduler
  - **Container runtime issues**: containerd/CRI-O failures
  - **Image issues**: Container crashes on startup

**Current handling:**

- ✅ **Container state monitoring**: When EOF received on log stream, `KubeLoggingWithReconnect()` gets fresh pod status and examines container state
- ✅ **Terminated state detection**: Checks `containerState.Terminated` to determine if container has stopped
- ✅ **Exit code inspection**: Reads `containerState.Terminated.ExitCode` to determine exit status
- ✅ **Reason classification**: Checks `containerState.Terminated.Reason` field and uses a whitelist approach:
  - **Whitelist reasons** `["Completed", "Error"]`: Container ran to normal completion (even if it exited with error code)
  - **All other reasons** (OOMKilled, Evicted, etc.): Execution was interrupted abnormally
- ✅ **Work state determination logic**:
  - Exit code 0 → WorkStateSucceeded
  - Exit code != 0 + reason in whitelist (`"Completed"` or `"Error"`) → WorkStateSucceeded (normal program completion with error)
  - Exit code != 0 + reason NOT in whitelist (e.g., `"OOMKilled"`, `"Evicted"`) → WorkStateFailed and sets `stdoutErr`
- ✅ **Error marking**: Sets `stdoutErr` only if execution interrupted (not for normal error completions)
- ✅ **Last line capture**: Attempts to write last line from log stream before container termination
- ✅ **Detailed logging**: Logs exit code, termination reason, and termination message for all non-zero exits

**Impact:** Container-level terminations are properly detected and classified. Work state determined by both exit code AND termination reason. This correctly distinguishes between:

- Programs that exit with non-zero status intentionally (marked as succeeded if reason is "Completed"/"Error")
- Containers killed by OOM, eviction, or other interruptions (marked as failed)

**Note:** OOMKilled is a **container-level event** that appears in `containerState.Terminated.Reason`, distinct from pod-level eviction events.

#### Other Containers in Pod Are Killed

**What happens:**

- Sidecar containers or init containers fail or are killed
- **Note**: Multi-container pods are only possible when using custom pod specs (Pod parameter). In normal mode (Image/Command/Params), the pod is created with a single container named "worker", so sidecars and init containers are not possible.

**Current handling:**

- ⚠️ **Limited detection**: Only checks `WorkerContainerName` container
- ⚠️ **No sidecar monitoring**: Doesn't check status of other containers in `pod.Status.ContainerStatuses`
- ⚠️ **No init container monitoring**: Doesn't check `pod.Status.InitContainerStatuses`
- ⚠️ **Pod phase impact**: If sidecar/init failures cause pod to fail, pod phase change is detected via watch
- ⚠️ **Init container failures**: May prevent pod from reaching Ready state, detected as timeout during `podRunningAndReady()` watch

**Impact:**

- If worker container unaffected by sidecar failure, job continues normally
- If pod fails due to sidecar/init failure, detected indirectly via pod phase (PodFailed) or timeout waiting for Ready state
- No visibility into which sidecar/init container caused the failure - only that the pod failed
- Custom pod specs allow multiple containers and init containers, but normal mode creates single-container pods only

### Invalid Input Handling

#### Invalid PodTemplate Provided

**What happens:**

- Invalid YAML/JSON, missing required fields, invalid container names, incompatible spec provided in pod template

**Current handling:**

- ✅ **YAML/JSON decoding**: Errors caught in `CreatePod()` when decoding `ked.KubePod`
- ✅ **Worker container validation**: Checks that container named "worker" exists
- ✅ **Required fields**: Kubernetes API validates pod spec during `Create()` call
- ⚠️ **Partial validation**: Only validates worker container exists, not other aspects
- ❌ **No pre-validation**: Invalid specs discovered only when creating pod

**Impact:** Job fails at pod creation with validation error. Error message includes decoding or validation details.

### Long-Running Job Scenarios

#### Job Running for 40+ Hours

**What happens:**

- Very long-running jobs that exceed normal timeouts and log stream limits

**Current handling:**

- ✅ **4-hour log stream timeout**: Kubernetes API closes log streams after 4 hours
- ✅ **Automatic reconnection**: `KubeLoggingWithReconnect()` detects EOF and reconnects with `sinceTime` to avoid duplicates
- ✅ **Timestamp-based deduplication**: `ProcessLogLine()` uses timestamps to skip duplicate lines
- ✅ **Context cancellation handling**: Checks `context.Canceled` during log reading
- ✅ **EOF with Running state**: When EOF is detected but container is still Running, the system continues attempting to reconnect indefinitely (no retry limit) using Fibonacci backoff. This handles both cases: 4-hour log stream timeouts and rapid state transitions to terminated.
- ✅ **Fibonacci backoff**: Uses `GetNextFibonacciValues()` for retry delay calculations (capped at 400 to prevent excessive delays, max sleep duration of 5 minutes)
- ⚠️ **No job timeout**: No maximum job duration enforced by Receptor itself
- ⚠️ **Context cancellation**: Depends on external context cancellation (e.g., from the work submission client)

**Impact:** Jobs can run indefinitely if context not canceled. Log streams automatically reconnect every 4 hours. When EOF occurs with a Running container, the system continues attempting reconnection indefinitely rather than failing, which improves handling of long-running jobs and 4-hour timeout scenarios.

#### Context Cancellation

**What happens:**

- Context is canceled, triggering cleanup of the work unit
- **Important distinction**:
  - **Pod startup failures** (before execution begins): `Cancel()` IS called automatically
  - **Errors during execution** (after pod running): Errors do NOT cancel context - they just mark job as failed and set error details
  - **Context cancellation during execution**: Only from explicit user action or Receptor shutdown, NOT from execution errors

**Cancellation triggers:**

- ✅ **User cancels work**: Via `receptorctl work cancel <unit-id>`
- ✅ **User releases work**: Via `receptorctl work release <unit-id>` (calls `Cancel()`)
- ✅ **Pod startup failure**: When `CreatePod()` fails (e.g., ErrPodFailed, ErrImagePullBackOff) - automatic cleanup
- ⚠️ **Receptor shutdown**: SIGTERM/SIGINT to the Receptor process (if signal handler cancels contexts)
- ❌ **NOT triggered by execution errors**: Container failures, OOMKilled, log stream errors, etc. do NOT cancel context - they mark job as failed but don't trigger `Cancel()`

**Current handling:**

- ✅ **Context check in log reading**: Detects `context.Canceled` and marks job as failed if not already in terminal state
- ✅ **Context propagation**: Uses `kw.GetContext()` throughout for cancellation propagation
- ✅ **Cancel() method**: Deletes pod via Kubernetes API and marks as `WorkStateCanceled`
- ⚠️ **No graceful shutdown**: No attempt to wait for current operation to complete or allow pod to finish
- ⚠️ **Immediate pod deletion**: Pod is deleted immediately, may interrupt running job and lose partial output

**Impact:**

- Job marked as `WorkStateCanceled`
- Pod deleted from Kubernetes cluster
- Partial output may be lost if job was mid-execution
- For user-initiated cancellation, this is intentional behavior
- For pod startup failures, this is automatic cleanup to remove failed pods

### Log and Disk I/O Errors

#### Log Returned by Kube API Is Too Long

**What happens:**

- Very large log outputs that exceed memory or streaming buffer limits

**Current handling:**

- ✅ **Streaming approach**: Uses streaming reads (`bufio.NewReader`) not loading all logs into memory
- ✅ **Line-by-line processing**: Processes one line at a time
- ⚠️ **No size limits**: No explicit maximum log size limits
- ⚠️ **Disk space**: Depends on available disk space for stdout file
- ⚠️ **Memory**: Should be safe due to streaming, but very long individual lines may cause issues

**AAP Controller capacity checks:**

- ✅ **Memory**: AAP controller checks memory capacity before starting jobs via `mem_capacity` algorithm
  - Jobs stay in "pending" state if insufficient memory capacity available
  - Reserves ~100MB per fork + 2GB for system services
- ❌ **Disk space**: AAP controller does NOT check disk space before starting jobs
  - No pre-flight validation of available disk space
  - Jobs can start successfully then fail mid-execution when disk fills
  - Work units write stdout to disk without size limits until disk is full

**Impact:** Large logs should work due to streaming, but **disk space exhaustion is a real operational risk**. Jobs will fail with "no space left on device" errors if disk fills during execution. Monitor disk usage on execution nodes, especially `/var/lib/awx` and `/tmp`.

#### Cannot Write Logs to Disk

**What happens:**

- Disk full, permission errors, filesystem errors when writing stdout file

**Current handling:**

- ✅ **Error detection**: Checks `stdout.Write()` errors
- ✅ **Error propagation**: Sets `stdoutErr` and logs error
- ✅ **Job failure**: Marks job as failed when write error occurs
- ⚠️ **No retry**: Write errors are not retried (assumed to be persistent)
- ⚠️ **Partial writes**: If write fails mid-stream, partial data may be in file

**Impact:** Job fails immediately on write error. Partial logs may be present in stdout file.

### Known Unknowns

These scenarios either have unclear handling or require further investigation:

1. **API Server Network Partitions**: What happens if network partitions between Receptor and API server during job execution?

   **What happens:**

   - Network connectivity is lost between Receptor and the Kubernetes API server (routing issues, network maintenance, infrastructure failures)
   - API calls fail with connection refused or timeout errors
   - Receptor cannot observe pod state changes during the partition

   **Current handling:**

   - ✅ **Retry logic**: `KubeLoggingWithReconnect()` retries getting the pod with Fibonacci backoff (default 5 retries, max 100)
   - ⚠️ **Limited retries**: With default settings (5 retries, 1s base timeout), total retry time is approximately 12-15 seconds. Partitions longer than this cause job failure
   - ⚠️ **Eventual consistency**: If the partition occurs while the pod transitions from Running to Terminated, Receptor may miss the state change and fail the job even if the pod completed successfully
   - ⚠️ **No infinite retry**: Retries are finite (default 5, max 100), so long partitions will cause job failure even if the pod is still running or completes successfully

   **Impact:** Network partitions longer than the retry window (default ~15 seconds) will cause job failure, potentially even if the pod completes successfully during the partition. The retry logic helps with transient issues but cannot handle extended partitions.

2. **Pod Template Mutations**: What if pod spec is mutated after creation by admission controllers or webhooks? (Unknown: Original spec used, mutations not tracked)

## Related Documentation

- [Receptor Work Submit Flow](receptor_work_submit_flow.md) - General work submission flow
- [Add Listener Backend](AddListenerBackend.md) - Backend connection handling
