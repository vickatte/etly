package etly

import (
	"github.com/viant/toolbox"
	"time"
)

//Transfer represents transfer rule
type Transfer struct {
	Name                 string
	Source               string
	SourceType           string //url,table
	SourceFormat         string //nd_json,json
	SourceExt            string
	SourceEncoding       string //gzip
	SourceDataType       string //name of source struct
	SourceDataTypeMatch  []*SourceDataType
	Target               string
	TargetType           string //url,table
	TargetEncoding       string //gzip
	TimeWindow           int    //how long back go in time - to be used with <dateFormat:XX> expression
	TimeWindowUnit       string //time unit: sec, min, hour, day
	MaxParallelTransfers int
	MaxTransfers         int
	MetaUrl              string
	Transformer          string //name of registered transformer
	Filter               string //name of registered filter predicate

	TimeFrequency     int
	TimeFrequencyUnit string
	nextRun           *time.Time
}

func (t *Transfer) scheduleNextRun(now time.Time) error {
	timeUnitFactor, err := timeUnitFactor(t.TimeFrequencyUnit)
	if err != nil {
		return err
	}
	var delta = time.Duration(timeUnitFactor * int64(t.TimeFrequency))
	nextRun := now.Add(delta)
	t.nextRun = &nextRun
	return nil
}

func (t *Transfer) String() string {
	return "[id: " + t.Name + ", Source: " + t.Source + ", Target: " + t.Target + "]"
}

//Clone creates a copy of the transfer
func (t *Transfer) Clone(source, target, metaUrl string) *Transfer {
	return &Transfer{
		Name:                 t.Name,
		Source:               source,
		SourceType:           t.SourceType,
		SourceExt:            t.SourceExt,
		SourceFormat:         t.SourceFormat,
		SourceEncoding:       t.SourceEncoding,
		SourceDataType:       t.SourceDataType,
		Target:               target,
		TargetType:           t.TargetType,
		TargetEncoding:       t.TargetEncoding,
		MetaUrl:              metaUrl,
		TimeWindow:           t.TimeWindow,
		TimeWindowUnit:       t.TimeWindowUnit,
		SourceDataTypeMatch:  t.SourceDataTypeMatch,
		MaxParallelTransfers: t.MaxParallelTransfers,
		MaxTransfers:         t.MaxTransfers,
		Transformer:          t.Transformer,
		Filter:               t.Filter,
	}
}

//SourceDataType represents a source data type matching rule,
type SourceDataType struct {
	MatchingFragment string
	DataType         string
}

//StorageConfig represents storage config to be used to register various storage schema protocols with storage namepsace
type StorageConfig struct {
	Namespace string
	Schema    string
	Config    string
}

//Config ETL config
type Config struct {
	Transfers []*Transfer
	Storage   []*StorageConfig
	Port      int
}

//NewConfigFromUrl creates a new config from URL
func NewConfigFromUrl(URL string) (*Config, error) {
	var result = &Config{}
	err := toolbox.LoadConfigFromUrl(URL, result)
	return result, err
}