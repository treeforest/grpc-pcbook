package serializer

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
)

// WriteProtobufToBinaryFile write protocol buffer message to binary file
func WriteProtobufToBinaryFile(message proto.Message, filename string) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("connot marshal proto.message to binary: %w", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("connot write binary data to file: %w", err)
	}

	return nil
}

// ReadProtobufFromBinaryFile read protocol buffer message from binary file
func ReadProtobufFromBinaryFile(filename string, message proto.Message) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("connot read binary data from file: %w", err)
	}

	err = proto.Unmarshal(data, message)
	if err != nil {
		return fmt.Errorf("connot unmarshal binary to proto message: %w", err)
	}

	return nil
}
