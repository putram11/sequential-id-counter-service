#!/bin/bash

# RabbitMQ initialization script
# This script sets up exchanges, queues, and bindings for the Sequential ID service

set -e

# Wait for RabbitMQ to be ready
echo "Waiting for RabbitMQ to be ready..."
until rabbitmqctl status > /dev/null 2>&1; do
    echo "RabbitMQ is not ready yet. Waiting..."
    sleep 2
done

echo "RabbitMQ is ready. Setting up exchanges and queues..."

# Declare exchange
rabbitmqctl eval 'rabbit_exchange:declare({resource, <<"/">>, exchange, <<"seq_exchange">>}, direct, true, false, false, []).'

# Declare main queue
rabbitmqctl eval 'rabbit_amqqueue:declare({resource, <<"/">>, queue, <<"seq_log_queue">>}, true, false, [], none).'

# Declare dead letter queue
rabbitmqctl eval 'rabbit_amqqueue:declare({resource, <<"/">>, queue, <<"seq_log_dlq">>}, true, false, [], none).'

# Bind queue to exchange
rabbitmqctl eval 'rabbit_binding:add({binding, {resource, <<"/">>, exchange, <<"seq_exchange">>}, <<"seq.log">>, {resource, <<"/">>, queue, <<"seq_log_queue">>}, []}).'

# Set queue policy for high availability (if clustering)
rabbitmqctl set_policy ha-seq "^seq_" '{"ha-mode":"all","ha-sync-mode":"automatic"}'

# Set TTL and dead letter policies
rabbitmqctl set_policy seq-ttl "^seq_log_queue$" '{"message-ttl":86400000,"dead-letter-exchange":"","dead-letter-routing-key":"seq_log_dlq"}'

echo "RabbitMQ setup completed successfully!"

# Show current setup
echo "Current exchanges:"
rabbitmqctl list_exchanges

echo "Current queues:"
rabbitmqctl list_queues

echo "Current bindings:"
rabbitmqctl list_bindings
