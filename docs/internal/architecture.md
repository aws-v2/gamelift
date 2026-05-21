# Architectural Overview

This document provides a high-level overview of the GameLift backend architecture, its core components, and data flows.

## Table of Contents
1. [System Architecture](#system-architecture)
2. [Identity & Authentication](#identity-authentication)
3. [Game Session Lifecycle](#game-session-lifecycle)
4. [Storage & ARN Conventions](#storage--arn-conventions)
5. [Log Aggregation & Observability](#log-aggregation--observability)

## 1. System Architecture

The GameLift backend is a microservices-based system designed for orchestrating game servers on virtualized compute resources.

- **API Service**: Entry point for clients (Unity/Godot) and management consoles.
- **Compute Service (EC2/Agent)**: Manages VM lifecycles and game process execution.
- **Storage Service (S3/MinIO)**: Stores game binaries and assets.
- **Messaging Layer (NATS)**: Asynchronous event bus for cross-service communication.

## 2. Identity & Authentication

We use a custom `AuthMiddleware` that enforces identity based on internal headers provided by the API Gateway or edge proxy.

- **Headers**:
    - `X-User-Id`: Identifies the user.
    - `X-User-Role`: Determines permissions.
- **Session Tokens**: Short-lived JWTs generated for game servers to authenticate with the backend via NATS or WebSocket.

## 3. Game Session Lifecycle

1. **InitUpload**: User requests to upload a game. We return a presigned URL.
2. **Provision**: Once uploaded, the S3 service notifies the Provisioning Service.
3. **VM Commissioning**: A VM is requested from the compute layer.
4. **Game Ready**: The agent in the VM notifies the backend when the game server is listening.

## 4. Storage & ARN Conventions

We use Amazon Resource Names (ARNs) to uniquely identify resources across the system.

- **Game ARN Format**: `arn:aws:s3:::gameliftgames-default/uploads/games/<game_id>/<version>/package`
- **Object Keys**: Assets are stored using a Content-Addressable Storage (CAS) logic when possible, or under the game-id prefix.

## 5. Log Aggregation & Observability

We use structured logging to ensure that all services contribute to a unified observability pipeline.

- **Logger**: Uber's `zap` (SugaredLogger).
- **Strategy**: See the [Logging Strategy](logging_strategy.md) for naming conventions and standard keys.
- **Correlation**: Every request should be traceable via `request_id`.
