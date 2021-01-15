package service

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sync"
)

// ImageStore is an interface to laptopStore laptop image
type ImageStore interface {
	// Save saves a new laptop image to the laptopStore
	Save(laptopID string, imageType string, imageData bytes.Buffer) (string, error)
}

// DiskImageStore stores images on disk add its info on memory
type DiskImageStore struct {
	mutex       sync.RWMutex
	imageFolder string
	images      map[string]*ImageInfo
}

// ImageInfo contains information of the laptop image
type ImageInfo struct {
	LaptopID string
	Type     string
	Path     string
}

// NewDiskImageStore returns a new DiskImageStore
func NewDiskImageStore(imageFolder string) *DiskImageStore {
	return &DiskImageStore{
		imageFolder: imageFolder,
		images:      make(map[string]*ImageInfo),
	}
}

// Save saves a new laptop image to the laptopStore
func (store *DiskImageStore) Save (
	laptopID string,
	imageType string,
	imageData bytes.Buffer,
	) (string, error) {
	imageID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("cannot generate image id:%w", err)
	}

	imagePath := filepath.Join(store.imageFolder, fmt.Sprintf("%s%s", imageID, imageType))

	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("cannot create image file: %w", err)
	}
	defer file.Close()

	_, err = imageData.WriteTo(file)
	if err != nil {
		return "", fmt.Errorf("cannot write image to file: %w", err)
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.images[imageID.String()] = &ImageInfo{
		LaptopID: laptopID,
		Type:     imageType,
		Path:     imagePath,
	}

	return imageID.String(), nil
}
