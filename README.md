# workflow

A workflow engine designed to orchestrate and execute containerized tasks. Automates processes with a focus on reliability, observability, and real-time feedback.

## Core Architecture

The architecture is built around three primary services that communicate through a message broker, ensuring loose coupling and independent scalability.

* **Coordinator**: The brain of the operation. The coordinator is responsible for managing the state of all tasks. It schedules new tasks, persists their state to the database, and publishes them to the message broker for workers to consume.
* **Worker**: The workhorse of the system. Workers subscribe to the task queues on the broker, pull tasks, and execute them within a containerized environment.
* **Streamer**: A dedicated service for real-time streaming. It subscribes to a separate queue and provides live container output to clients.

## Stack

* **Runner (usually Docker)**: Tasks are executed in isolated containers.
* **Message Broker (e.g., RabbitMQ)**: Provides a reliable communication channel for all inter-service messaging.
* **Storage (WIP)**: Stores the complete state of all tasks for durability and historical access.
* **Observability (OTEL)**: OTEL is integrated throughout the services to provide unified, end-to-end tracing, metrics, and logging.
