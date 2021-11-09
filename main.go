package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"

	fssz "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"
	log "github.com/sirupsen/logrus"

	ethpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
)

var fixturePath string
var fixtureType string

func init() {
	flag.StringVar(&fixturePath, "path", "", "path to spectests")
	flag.StringVar(&fixtureType, "type", "", "Name of type to test")
	flag.Parse()
}

func main() {
	log.Info(fmt.Sprintf("path=%s, type=%s", fixturePath, fixtureType))
	_, err := SSZObjectFromName(fixtureType)
	if err != nil {
		log.Fatalf("Couldn't find type for name %s", fixtureType)
	}
	tcs, err := findTestCases(fixturePath, map[string]struct{}{fixtureType: struct{}{}})
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range tcs {
		log.Info(t.path)
		v, err := SSZObjectFromName(t.typeName)
		if err != nil {
			log.Fatalf("Couldn't find type for name %s", t.typeName)
		}

		b, err := t.MarshaledBytes()
		if err != nil {
			log.Fatalf("Failed to get bytes for test case path=%s, with err=%s", t.path, err)
		}

		err = v.UnmarshalSSZ(b)
		if err != nil {
			log.Fatalf("Failed to unmarshal bytes for test case path=%s, with err=%s", t.path, err)
		}
		bb, err := v.MarshalSSZ()
		if err != nil {
			log.Fatalf("Failed to marshal bytes for test case path=%s, with err=%s", t.path, err)
		}
		if !bytes.Equal(b, bb) {
			log.Fatalf("Marshaled bytes do not equal fixture")
		}
		htr, err := v.HashTreeRoot()
		if err != nil {
			log.Fatalf("Unable to compute HTR, err=%s", err)
		}
		fmt.Printf("htr = %#x\n", htr)
	}
}

type SSZ interface {
	fssz.Unmarshaler
	fssz.Marshaler
	fssz.HashRoot
}

type SSZRoots struct {
	Root        string `json:"root"`
	SigningRoot string `json:"signing_root"`
}

type SSZValue struct {
	Message   json.RawMessage `json:"message"`
	Signature string          `json:"signature"` // hex encoded '0x...'
}

type TestCase struct {
	path     string
	config   string
	phase    string
	typeName string
	caseId   string
}

func (tc *TestCase) MarshaledBytes() ([]byte, error) {
	fh, err := os.Open(path.Join(tc.path, "serialized.ssz_snappy"))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	buf := bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(fh)
	return snappy.Decode(nil, buf.Bytes())
}

func (tc *TestCase) Value() (*SSZValue, error) {
	fh, err := os.Open(path.Join(tc.path, "value.yaml"))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	d := json.NewDecoder(fh)
	v := &SSZValue{}
	err = d.Decode(v)
	return v, err
}

func (tc *TestCase) Roots() (*SSZRoots, error) {
	fh, err := os.Open(path.Join(tc.path, "roots.yaml"))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	d := json.NewDecoder(fh)
	r := &SSZRoots{}
	err = d.Decode(r)
	return r, err
}

func findTestCases(path string, want map[string]struct{}) ([]*TestCase, error) {
	var re = regexp.MustCompile(`.*\/tests\/(mainnet|minimal)\/(altair|merge|phase0)\/ssz_static\/(.*)\/ssz_random\/(case_\d+)`)
	tcs := make([]*TestCase, 0)
	testCaseFromPath := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Error(err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		parts := re.FindStringSubmatch(path)
		if len(parts) != 5 {
			return nil
		}
		tc := &TestCase{
			path:     path,
			config:   parts[1],
			phase:    parts[2],
			typeName: parts[3],
			caseId:   parts[4],
		}
		if tc.config == "" || tc.phase == "" || tc.typeName == "" || tc.caseId == "" {
			return nil
		}
		if _, ok := want[tc.typeName]; !ok {
			return nil
		}
		tcs = append(tcs, tc)
		return nil
	}
	err := filepath.WalkDir(path, testCaseFromPath)

	return tcs, err
}

func SSZObjectFromName(name string) (SSZ, error) {
	var obj SSZ
	switch name {
	case "ExecutionPayload":
		obj = &ethpb.ExecutionPayload{}
	case "ExecutionPayloadHeader":
		obj = &ethpb.ExecutionPayloadHeader{}
	case "Attestation":
		obj = &ethpb.Attestation{}
	case "AttestationData":
		obj = &ethpb.AttestationData{}
	case "AttesterSlashing":
		obj = &ethpb.AttesterSlashing{}
	case "AggregateAndProof":
		obj = &ethpb.AggregateAttestationAndProof{}
	case "BeaconBlock":
		obj = &ethpb.BeaconBlockMerge{}
	case "BeaconBlockBody":
		obj = &ethpb.BeaconBlockBodyMerge{}
	case "BeaconBlockHeader":
		obj = &ethpb.BeaconBlockHeader{}
	case "BeaconState":
		obj = &ethpb.BeaconStateMerge{}
	case "Checkpoint":
		obj = &ethpb.Checkpoint{}
	case "Deposit":
		obj = &ethpb.Deposit{}
	case "DepositMessage":
		obj = &ethpb.DepositMessage{}
	case "DepositData":
		obj = &ethpb.Deposit_Data{}
	case "Eth1Data":
		obj = &ethpb.Eth1Data{}
	case "Fork":
		obj = &ethpb.Fork{}
	case "ForkData":
		obj = &ethpb.ForkData{}
	case "HistoricalBatch":
		obj = &ethpb.HistoricalBatch{}
	case "IndexedAttestation":
		obj = &ethpb.IndexedAttestation{}
	case "PendingAttestation":
		obj = &ethpb.PendingAttestation{}
	case "ProposerSlashing":
		obj = &ethpb.ProposerSlashing{}
	case "SignedAggregateAndProof":
		obj = &ethpb.SignedAggregateAttestationAndProof{}
	case "SignedBeaconBlock":
		obj = &ethpb.SignedBeaconBlockMerge{}
	case "SignedBeaconBlockHeader":
		obj = &ethpb.SignedBeaconBlockHeader{}
	case "SignedVoluntaryExit":
		obj = &ethpb.SignedVoluntaryExit{}
	case "SigningData":
		obj = &ethpb.SigningData{}
	case "Validator":
		obj = &ethpb.Validator{}
	case "VoluntaryExit":
		obj = &ethpb.VoluntaryExit{}
	case "SyncCommitteeMessage":
		obj = &ethpb.SyncCommitteeMessage{}
	case "SyncCommitteeContribution":
		obj = &ethpb.SyncCommitteeContribution{}
	case "ContributionAndProof":
		obj = &ethpb.ContributionAndProof{}
	case "SignedContributionAndProof":
		obj = &ethpb.SignedContributionAndProof{}
	case "SyncAggregate":
		obj = &ethpb.SyncAggregate{}
	case "SyncAggregatorSelectionData":
		obj = &ethpb.SyncAggregatorSelectionData{}
	case "SyncCommittee":
		obj = &ethpb.SyncCommittee{}
	default:
		return nil, errors.New("type not found")
	}
	return obj, nil
}
