package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/samber/lo"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.uber.org/zap"
)

func NewS3Client(ctx context.Context, config *Config, lg *zap.Logger) *s3.Client {
	cfg, err := awsCfg.LoadDefaultConfig(
		ctx,
		awsCfg.WithRegion(config.Region),
		awsCfg.WithCredentialsProvider(aws.CredentialsProviderFunc(func(_ context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     config.AccessKeyId,
				SecretAccessKey: config.SecretAccessKey,
			}, nil
		})),
		awsCfg.WithLogger(getDefaultAwsLoggerFunc(lg)),
		awsCfg.WithLogConfigurationWarnings(true),
		awsCfg.WithClientLogMode(getDefaultAwsLogMode(lg)),
	)
	if err != nil {
		panic(err)
	}

	otelaws.AppendMiddlewares(&cfg.APIOptions)
	return s3.NewFromConfig(
		cfg,
		func(options *s3.Options) {
			// hack to resolve endpoint
			// TODO: resolve endpoint with multi-region
			options.BaseEndpoint = aws.String(config.Url)
			options.UsePathStyle = true
		},
	)
}

func CreateS3BucketIfNotExists(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	buckets, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return err
	}
	if !lo.ContainsBy(buckets.Buckets, func(item types.Bucket) bool {
		return item.Name != nil && *item.Name == bucketName
	}) {
		_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
	}
	return err
}
