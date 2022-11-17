package main

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/huantt/kafka-dump/impl"
	"github.com/huantt/kafka-dump/pkg/kafka_utils"
	"github.com/huantt/kafka-dump/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	if err := run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	var logLevel string
	var filePath string
	var kafkaServers string
	var kafkaUsername string
	var kafkaPassword string
	var kafkaSecurityProtocol string
	var kafkaSASKMechanism string
	var kafkaGroupID string
	var topics *[]string
	rootCmd := &cobra.Command{}
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "info", "Log level")
	logConfig := log.Config{Level: logLevel, Format: "text"}
	logConfig.Build()

	// Init count parquet number of rows command
	countParquetNumberOfRowsCmd := cobra.Command{
		Use: "count-parquet-rows",
		Run: func(cmd *cobra.Command, args []string) {
			parquetReader, err := impl.NewParquetReader(filePath)
			if err != nil {
				panic(errors.Wrap(err, "Unable to init parquet file reader"))
			}
			log.Infof("Number of rows: %d", parquetReader.GetNumberOfRows())
		},
	}
	countParquetNumberOfRowsCmd.Flags().StringVarP(&filePath, "file", "f", "", "File path (required)")
	err := countParquetNumberOfRowsCmd.MarkFlagRequired("file")
	if err != nil {
		return err
	}

	// Init export command
	exportCmd := cobra.Command{
		Use: "export",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Output destination: %s", filePath)
			parquetWriter, err := impl.NewParquetWriter(filePath)
			if err != nil {
				panic(errors.Wrap(err, "Unable to init parquet file writer"))
			}

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
			exporter, err := impl.NewExporter(consumer, *topics, parquetWriter)
			if err != nil {
				panic(errors.Wrap(err, "Failed to init exporter"))
			}

			err = exporter.Run()
			if err != nil {
				panic(errors.Wrap(err, "Error while running exporter"))
			}
		},
	}
	exportCmd.Flags().StringVarP(&filePath, "file", "f", "", "Output file path (required)")
	exportCmd.Flags().StringVarP(&kafkaServers, "kafka-servers", "", "", "Kafka servers string")
	exportCmd.Flags().StringVarP(&kafkaUsername, "kafka-username", "", "", "Kafka username")
	exportCmd.Flags().StringVarP(&kafkaPassword, "kafka-password", "", "", "Kafka password")
	exportCmd.Flags().StringVarP(&kafkaSASKMechanism, "kafka-sasl-mechanism", "", "", "Kafka password")
	exportCmd.Flags().StringVarP(&kafkaSecurityProtocol, "kafka-security-protocol", "", "", "Kafka security protocol")
	exportCmd.Flags().StringVarP(&kafkaGroupID, "kafka-group-id", "", "", "Kafka consumer group ID")
	topics = exportCmd.Flags().StringArrayP("kafka-topics", "", nil, "Kafka topics")

	exportCmd.MarkFlagsRequiredTogether("kafka-username", "kafka-password", "kafka-sasl-mechanism", "kafka-security-protocol")
	err = exportCmd.MarkFlagRequired("file")
	if err != nil {
		return err
	}

	// Init import command
	importCmd := cobra.Command{
		Use: "import",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Input file: %s", filePath)
			parquetReader, err := impl.NewParquetReader(filePath)
			if err != nil {
				panic(errors.Wrap(err, "Unable to init parquet file reader"))
			}
			kafkaProducerConfig := kafka_utils.Config{
				BootstrapServers: kafkaServers,
				SecurityProtocol: kafkaSecurityProtocol,
				SASLMechanism:    kafkaSASKMechanism,
				SASLUsername:     kafkaUsername,
				SASLPassword:     kafkaPassword,
			}
			producer, err := kafka_utils.NewProducer(kafkaProducerConfig)
			if err != nil {
				panic(errors.Wrap(err, "Unable to create producer"))
			}
			queueBufferingMaxMessages := kafka_utils.DefaultQueueBufferingMaxMessages
			if kafkaProducerConfig.QueueBufferingMaxMessages > 0 {
				queueBufferingMaxMessages = kafkaProducerConfig.QueueBufferingMaxMessages
			}
			deliveryChan := make(chan kafka.Event, queueBufferingMaxMessages)
			go func() { // Tricky: kafka require specific deliveryChan to use Flush function
				for e := range deliveryChan {
					m := e.(*kafka.Message)
					if m.TopicPartition.Error != nil {
						panic(fmt.Sprintf("Failed to deliver message: %v\n", m.TopicPartition))
					} else {
						log.Debugf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
							*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
					}
				}
			}()
			importer := impl.NewImporter(producer, deliveryChan, parquetReader)
			if err != nil {
				panic(errors.Wrap(err, "Unable to init importer"))
			}

			err = importer.Run()
			if err != nil {
				panic(errors.Wrap(err, "Error while running importer"))
			}
		},
	}
	importCmd.Flags().StringVarP(&filePath, "file", "f", "", "Output file path (required)")
	importCmd.Flags().StringVarP(&kafkaServers, "kafka-servers", "", "", "Kafka servers string")
	importCmd.Flags().StringVarP(&kafkaUsername, "kafka-username", "", "", "Kafka username")
	importCmd.Flags().StringVarP(&kafkaPassword, "kafka-password", "", "", "Kafka password")
	importCmd.Flags().StringVarP(&kafkaSASKMechanism, "kafka-sasl-mechanism", "", "", "Kafka password")
	importCmd.Flags().StringVarP(&kafkaSecurityProtocol, "kafka-security-protocol", "", "", "Kafka security protocol")
	importCmd.MarkFlagsRequiredTogether("kafka-username", "kafka-password", "kafka-sasl-mechanism", "kafka-security-protocol")
	err = importCmd.MarkFlagRequired("file")
	if err != nil {
		return err
	}

	rootCmd.AddCommand(
		&exportCmd,
		&importCmd,
		&countParquetNumberOfRowsCmd,
	)

	return rootCmd.Execute()
}
