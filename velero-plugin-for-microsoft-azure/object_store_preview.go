package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/pkg/errors"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

const (
	blob_url_suffix = "https://%s.blob.core.windows.net"
)

type ObjectStorePreview struct {
	pipeline *pipeline.Pipeline
	service  *azblob.ServiceURL
}

func (o *ObjectStorePreview) Init(config map[string]string) error {
	if err := veleroplugin.ValidateObjectStoreConfigKeys(config,
		resourceGroupConfigKey,
		storageAccountConfigKey,
		subscriptionIDConfigKey,
		storageAccountKeyEnvVarConfigKey,
	); err != nil {
		return err
	}

	storageAccountKey, _, err := getStorageAccountKey(config)
	if err != nil {
		return err
	}

	cred, err := azblob.NewSharedKeyCredential(config[storageAccountConfigKey], storageAccountKey)
	if err != nil {
		return err
	}

	u, _ := url.Parse(fmt.Sprintf(blob_url_suffix, config[storageAccountConfigKey]))
	if err != nil {
		return err
	}

	pipeline := azblob.NewPipeline(cred, azblob.PipelineOptions{})
	service := azblob.NewServiceURL(*u, pipeline)

	o.pipeline = &pipeline
	o.service = &service

	return nil
}

func (o *ObjectStorePreview) PutObject(bucket, key string, body io.Reader) error {
	container := o.service.NewContainerURL(bucket)
	blobURL := container.NewBlockBlobURL(key)
	response, err := azblob.UploadStreamToBlockBlob(context.Background(), body, blobURL, azblob.UploadStreamToBlockBlobOptions{})
	_ = response

	if err != nil {
		return err
	}
	return nil
}

func (o *ObjectStorePreview) ObjectExists(bucket, key string) (bool, error) {
	ctx := context.Background()
	container := o.service.NewContainerURL(bucket)
	blob := container.NewBlobURL(key)
	_, err := blob.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})

	if err == nil {
		return true, err
	}

	if storageErr, ok := err.(azblob.StorageError); ok {
		if storageErr.Response().StatusCode == 404 {
			return false, nil
		}
	}

	return false, err
}

func (o *ObjectStorePreview) GetObject(bucket, key string) (io.ReadCloser, error) {
	container := o.service.NewContainerURL(bucket)
	blobURL := container.NewBlockBlobURL(key)
	response, err := blobURL.Download(context.TODO(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, err
	}

	return response.Body(azblob.RetryReaderOptions{}), nil
}

func (o *ObjectStorePreview) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	return make([]string, 0), nil // This function is not implemented.
}

func (o *ObjectStorePreview) ListObjects(bucket, prefix string) ([]string, error) {
	var objects []string
	ctx := context.Background()

	container := o.service.NewContainerURL(bucket)

	marker := azblob.Marker{}
	for marker.NotDone() {
		listBlob, err := container.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})

		if err != nil {
			return nil, err
		}
		marker = listBlob.NextMarker

		for _, blobInfo := range listBlob.Segment.BlobItems {
			if prefix == "" || strings.Index(blobInfo.Name, prefix) == 0 {
				objects = append(objects, blobInfo.Name)
			}
		}
	}
	return objects, nil
}

func (o *ObjectStorePreview) DeleteObject(bucket string, key string) error {
	container := o.service.NewContainerURL(bucket)
	blobURL := container.NewBlockBlobURL(key)
	_, err := blobURL.Delete(context.Background(), azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	if err != nil {
		return err
	}
	return nil
}

func (o *ObjectStorePreview) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return "", errors.New("Not Implemented")
}
