package etly

import (
	"bytes"
	"fmt"
	"github.com/viant/toolbox"
	"sync/atomic"
)

type decodingError struct {
	error error
	count int
}

func transferRecord(state map[string]interface{}, predicate toolbox.Predicate, dataTypeProvider func() interface{}, encoded []byte, transformer Transformer, transfer *Transfer, transformedTargets map[string]*TargetTransformation, task *TransferTask, decodingError *decodingError, contentEnricher ContentEnricher, request *TransferObjectRequest) error {
	record := dataTypeProvider()
	err := decodeJSONTarget(encoded, record)
	if err != nil {
		decodingError.count++
		decodingError.error = fmt.Errorf("failed to decode json (%v times): %v, %s", decodingError.count, err, encoded)
		if transfer.MaxErrorCounts != nil && decodingError.count >= *transfer.MaxErrorCounts {
			return fmt.Errorf("reached max errors %v", decodingError)
		}
		return nil
	}

	if contentEnricher != nil {
		record, err = contentEnricher(record, request)
		if err != nil {
			return fmt.Errorf("failed to add content %v", err)
		}
	}

	if predicate == nil || predicate.Apply(record) {
		if payloadAccessor, ok := record.(PayloadAccessor); ok {
			payloadAccessor.SetPayload(string(encoded))
		}

		var target = record
		if transformer != nil {
			target, err = transformer(record)
			if err != nil {
				return fmt.Errorf("failed to transform %v", err)
			}
		}
		buf := new(bytes.Buffer)
		err = encodeJSONSource(buf, target)
		if err != nil {
			return err
		}
		transformedObject := bytes.Replace(buf.Bytes(), []byte("\n"), []byte(""), -1)
		targetKey, err := getTargetKey(transfer, record, target, state)
		if err != nil {
			return err
		}

		_, found := transformedTargets[targetKey]
		if !found {
			targetTransfer := transfer.Clone()
			targetTransfer.Target.Name = targetKey
			targetTransfer.Meta.Name, err = expandWorkerVariables(targetTransfer.Meta.Name, transfer, record, target)
			if err != nil {
				return err
			}
			transformedTargets[targetKey] = &TargetTransformation{
				targetRecords: make([]string, 0),
				ProcessedTransfer: &ProcessedTransfer{
					Transfer: targetTransfer,
				},
			}
		}
		transformedTargets[targetKey].targetRecords = append(transformedTargets[targetKey].targetRecords, string(transformedObject))
		transformedTargets[targetKey].RecordProcessed++
		atomic.AddInt32(&task.Progress.RecordProcessed, 1)
		transformedTargets[targetKey].RecordErrors = decodingError.count
	} else {
		task.Progress.RecordSkipped++
	}
	return nil
}
