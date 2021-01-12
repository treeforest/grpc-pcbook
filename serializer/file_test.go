package serializer

import (
	"github.com/golang/protobuf/proto"
	"github.com/treeforest/grpc-pcbook/pb"
	"github.com/treeforest/grpc-pcbook/sample"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFileSerializer(t *testing.T) {
	t.Parallel()

	binaryFile := "../tmp/laptop.bin"
	jsonFIle := "../tmp/laptop.json"

	laptop := sample.NewLaptop()
	err := WriteProtobufToBinaryFile(laptop, binaryFile)
	require.NoError(t, err)

	laptop2 := &pb.Laptop{}
	err = ReadProtobufFromBinaryFile(binaryFile, laptop2)
	require.NoError(t, err)

	require.True(t, proto.Equal(laptop, laptop2))

	err = WriteProtobufToJsonFile(laptop, jsonFIle)
	require.NoError(t, err)
}