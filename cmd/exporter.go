package cmd

import (
	"fmt"
	"github.com/huantt/kafka-dump/impl"
	"github.com/huantt/kafka-dump/pkg/kafka_utils"
	"github.com/huantt/kafka-dump/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"sync"
	"time"
)

func CreateExportCommand() (*cobra.Command, error) {
	var filePath string
	var kafkaServers string
	var kafkaUsername string
	var kafkaPassword string
	var kafkaSecurityProtocol string
	var kafkaSASKMechanism string
	var kafkaGroupID string
	var topics *[]string
	var exportLimitPerFile uint64
	var maxWaitingSecondsForNewMessage int
	var concurrentConsumers = 1

	command := cobra.Command{
		Use: "export",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Limit: %d - Concurrent consumers: %d", exportLimitPerFile, concurrentConsumers)
			kafkaConsumerConfig := kafka_utils.Config{
				BootstrapServers: kafkaServers,
				SecurityProtocol: kafkaSecurityProtocol,
				SASLMechanism:    kafkaSASKMechanism,
				SASLUsername:     kafkaUsername,
				SASLPassword:     kafkaPassword,
				GroupId:          kafkaGroupID,
			}
			consumer, err := kafka_utils.NewConsumer(kafkaConsumerConfig)
			if err != nil {
				panic(errors.Wrap(err, "Unable to init consumer"))
			}
			maxWaitingTimeForNewMessage := time.Duration(maxWaitingSecondsForNewMessage) * time.Second
			options := &impl.Options{
				Limit:                       exportLimitPerFile,
				MaxWaitingTimeForNewMessage: &maxWaitingTimeForNewMessage,
			}

			var wg sync.WaitGroup
			wg.Add(concurrentConsumers)
			for i := 0; i < concurrentConsumers; i++ {
				go func(workerID int) {
					defer wg.Done()
					for true {
						outputFilePath := filePath
						if exportLimitPerFile > 0 {
							outputFilePath = fmt.Sprintf("%s.%d", filePath, time.Now().UnixMilli())
						}
						log.Infof("[Worker-%d] Exporting to: %s", workerID, outputFilePath)
						parquetWriter, err := impl.NewParquetWriter(outputFilePath)
						if err != nil {
							panic(errors.Wrap(err, "Unable to init parquet file writer"))
						}
						exporter, err := impl.NewExporter(consumer, *topics, parquetWriter, options)
						if err != nil {
							panic(errors.Wrap(err, "Failed to init exporter"))
						}

						exportedCount, err := exporter.Run()
						if err != nil {
							panic(errors.Wrap(err, "Error while running exporter"))
						}
						log.Infof("[Worker-%d] Exported %d messages", workerID, exportedCount)
						if exportLimitPerFile == 0 || exportedCount < exportLimitPerFile {
							log.Infof("[Worker-%d] Finished!", workerID)
							return
						}
					}
				}(i)
			}
			wg.Wait()
		},
	}
	command.Flags().StringVarP(&filePath, "file", "f", "", "Output file path (required)")
	command.Flags().StringVar(&kafkaServers, "kafka-servers", "", "Kafka servers string")
	command.Flags().StringVar(&kafkaUsername, "kafka-username", "", "Kafka username")
	command.Flags().StringVar(&kafkaPassword, "kafka-password", "", "Kafka password")
	command.Flags().StringVar(&kafkaSASKMechanism, "kafka-sasl-mechanism", "", "Kafka password")
	command.Flags().StringVar(&kafkaSecurityProtocol, "kafka-security-protocol", "", "Kafka security protocol")
	command.Flags().StringVar(&kafkaGroupID, "kafka-group-id", "", "Kafka consumer group ID")
	command.Flags().Uint64Var(&exportLimitPerFile, "limit", 0, "Supports file splitting. Files are split by the number of messages specified")
	command.Flags().IntVar(&maxWaitingSecondsForNewMessage, "max-waiting-seconds-for-new-message", 30, "Max waiting seconds for new message, then this process will be marked as finish. Set -1 to wait forever.")
	command.Flags().IntVar(&concurrentConsumers, "concurrent-consumers", 1, "Number of concurrent consumers")
	topics = command.Flags().StringArray("kafka-topics", nil, "Kafka topics")
	command.MarkFlagsRequiredTogether("kafka-username", "kafka-password", "kafka-sasl-mechanism", "kafka-security-protocol")
	err := command.MarkFlagRequired("file")
	if err != nil {
		return nil, err
	}
	return &command, nil
}