package chaincode

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-protos-go/peer"
	"log"
	"unsafe"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

const syntax = "proto3"

// GetName returns the name of the contract
func (s *SmartContract) GetName() string {
	return "Practice_SmartContract"
}

// Asset describes basic details of what makes up a simple asset
type Asset struct {
	ID             string `json:"ID"`
	Color          string `json:"color"`
	Size           int    `json:"size"`
	Owner          string `json:"owner"`
	AppraisedValue int    `json:"appraisedValue"`
}

// InitLedger adds a base set of assets to the ledger
func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	assets := []Asset{
		{ID: "asset1", Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300},
		{ID: "asset2", Color: "red", Size: 5, Owner: "Brad", AppraisedValue: 400},
		{ID: "asset3", Color: "green", Size: 10, Owner: "Jin Soo", AppraisedValue: 500},
		{ID: "asset4", Color: "yellow", Size: 10, Owner: "Max", AppraisedValue: 600},
		{ID: "asset5", Color: "black", Size: 15, Owner: "Adriana", AppraisedValue: 700},
		{ID: "asset6", Color: "white", Size: 15, Owner: "Michel", AppraisedValue: 800},
	}

	for _, asset := range assets {
		assetJSON, err := json.Marshal(asset)
		if err != nil {
			return err
		}

		err = ctx.GetStub().PutState(asset.ID, assetJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state. %v", err)
		}
	}

	return nil
}

// CreateAsset issues a new asset to the world state with given details.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", id)
	}

	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// ReadAsset returns the asset stored in the world state with given id.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", id)
	}

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

// UpdateAsset updates an existing asset in the world state with provided parameters.
func (s *SmartContract) UpdateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	// overwriting original asset with new asset
	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// DeleteAsset deletes an given asset from the world state.
func (s *SmartContract) DeleteAsset(ctx contractapi.TransactionContextInterface, id string) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	return ctx.GetStub().DelState(id)
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return assetJSON != nil, nil
}

// TransferAsset updates the owner field of asset with given id in world state.
func (s *SmartContract) TransferAsset(ctx contractapi.TransactionContextInterface, id string, newOwner string) error {
	asset, err := s.ReadAsset(ctx, id)
	if err != nil {
		return err
	}

	asset.Owner = newOwner
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// GetAllAssets returns all assets found in world state
func (s *SmartContract) GetAllAssets(ctx contractapi.TransactionContextInterface) ([]*Asset, error) {
	// range query with empty string for startKey and endKey does an
	// open-ended query of all assets in the chaincode namespace.
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var assets []*Asset
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var asset Asset
		err = json.Unmarshal(queryResponse.Value, &asset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}

	return assets, nil
}

// SomeStubMethod stub其他的无法通过mock方式测试的方法练习
func (s *SmartContract) SomeStubMethod(ctx contractapi.TransactionContextInterface, assetID string) error {
	stub := ctx.GetStub()
	// stub.GetArgs()和stub.GetStringArgs()都是获取调用链码时的入参，第一个参数时方法名，后面的参数是这个方法的参数的信息,如下：
	// 2021/01/25 08:06:32 stub.GetArgs(),i=0, arg=Practice_SmartContract:SomeStubMethod
	//2021/01/25 08:06:32 stub.GetArgs(),i=1, arg=asset1
	for i, arg := range stub.GetArgs() {
		log.Printf("stub.GetArgs(),i=%d, arg=%s", i, byteToString(arg))
	}
	for i, arg := range stub.GetStringArgs() {
		log.Printf("stub.GetStringArgs(),i=%d, arg=%s", i, arg)
	}
	binding, err := stub.GetBinding()
	if err != nil {
		return err
	}
	log.Printf("stub.GetBinding()=%s", byteToString(binding))
	for k, v := range stub.GetDecorations() {
		log.Printf("stub.GetDecorations(), k=%s, v=%s", k, byteToString(v))
	}
	// stub.GetCreator()返回的是证书，如过是组织s2.supply.com的管理员发起的交易，则此处获得的是：Admin@s2.supply.com-cert.pem
	creator, err := stub.GetCreator()
	if err != nil {
		return err
	}
	log.Printf("stub.GetCreator()=%s", byteToString(creator))
	// 已经签名的提议，包含以下内容：
	// 1.通道名称
	// 2.链码名称
	// 3.发起交易的组织名称
	// 4.发起交易的人的证书
	// 5.调用链码时的入参：方法名，参数等
	// stub.GetSignedProposal().GetProposalBytes()的信息如下：
	//2021/01/25 08:06:32 stub.GetSignedProposal().GetProposalBytes()=
	//�
	//v��������"alljoinchannel*@252b6bbd22eeaf2193cdbc86fe7bd9fa257e33a6209a5da7d81dcc41b8bb1b9d:secured_supply�
	//�
	//GylSOrg2MSP�-----BEGIN CERTIFICATE-----
	//MIICETCCAbegAwIBAgIRAJw2YUKkmyKusGHm33D7LhkwCgYIKoZIzj0EAwIwbTEL
	//MAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG
	//cmFuY2lzY28xFjAUBgNVBAoTDXMyLnN1cHBseS5jb20xGTAXBgNVBAMTEGNhLnMy
	//LnN1cHBseS5jb20wHhcNMjEwMTA3MDgzMTAwWhcNMzEwMTA1MDgzMTAwWjBYMQsw
	//CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
	//YW5jaXNjbzEcMBoGA1UEAwwTQWRtaW5AczIuc3VwcGx5LmNvbTBZMBMGByqGSM49
	//AgEGCCqGSM49AwEHA0IABJ6An5vHmug1YBIUXKuD50ZJ79TiwDkW5uEr2ZkXU5Em
	//XwVlxwCOKpfqKOr1Xdk0DWMlAQPQIxeXktdVBJxFc4KjTTBLMA4GA1UdDwEB/wQE
	//AwIHgDAMBgNVHRMBAf8EAjAAMCsGA1UdIwQkMCKAIGO9q5qcp089i7bDqwyxRYdg
	//aX65Bvs4X5wCsXWbxj37MAoGCCqGSM49BAMCA0gAMEUCIQCRBC/uF8ooaLQzSDo6
	//e5+4UbBqjSi5MUy3IYfVrM5tHQIgaGHKXcKZY7q0Txs6LsbtayW6kWPOAee6Z1W8
	//top2VDc=
	//-----END CERTIFICATE-----
	//�w�}dȧC>�v�@�El�S����I
	//G
	//Esecured_supply/
	//%Practice_SmartContract:SomeStubMethod
	//asset1
	proposal, err := stub.GetSignedProposal()
	if err != nil {
		return err
	}
	log.Printf("stub.GetSignedProposal()=%#v", proposal)
	bytes := proposal.GetProposalBytes()
	log.Printf("stub.GetSignedProposal().GetProposalBytes()=%s", byteToString(bytes))
	p := &peer.Proposal{}
	err = proto.Unmarshal(bytes, p)
	if err != nil {
		return err
	}
	log.Printf("stub.GetSignedProposal().GetProposalBytes(),proto.Unmarshal=%#v", p)
	//headerBytes:= p.GetHeader()
	//header := &peer.ChaincodeHeaderExtension{}
	//err = proto.Unmarshal(headerBytes, header)
	//if err != nil {
	//	return err
	//}
	//log.Printf("stub.GetSignedProposal().GetProposalBytes()-Proposal-GetHeader()=%#v", header)
	//payloadBytes := p.GetPayload()
	//payload := &peer.ChaincodeProposalPayload{}
	//err = proto.Unmarshal(payloadBytes, payload)
	//if err != nil {
	//	return err
	//}
	//log.Printf("stub.GetSignedProposal().GetProposalBytes()-Proposal-GetPayload()=%#v", payload)
	log.Printf("stub.GetSignedProposal().GetSignature()=%s", byteToString(proposal.GetSignature()))

	// 设置一个Event
	if err := stub.SetEvent("hello event", []byte("hello")); err != nil {
		return err
	}
	//2021/01/25 10:22:57 stub.GetHistoryForKey(asset1), next=&queryresult.KeyModification{
	//TxId:"f251ce5352e294cd628fc0b5d09271ebe8253b41d66069c164195fe2783c3adc",
	//Value:[]uint8{0x7b, 0x22, 0x49, 0x44, 0x22, 0x3a, 0x22, 0x61, 0x73, 0x73, 0x65, 0x74, 0x31, 0x22, 0x2c
	//, 0x22, 0x63, 0x6f, 0x6c, 0x6f, 0x72, 0x22, 0x3a, 0x22, 0x62, 0x6c, 0x75, 0x65, 0x22, 0x2c, 0x22, 0x73, 0x69, 0x7a, 0x65, 0x22, 0x3a, 0x35, 0x2c, 0x22, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x22, 0x3a, 0x22, 0x54, 0x6f, 0x6d, 0x6f, 0x6b, 0x6f, 0x22, 0x2c, 0x22, 0x61, 0x70, 0x70, 0x72, 0x61, 0x69, 0x73, 0x65, 0x64, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x3a, 0x33, 0x30, 0x30, 0x7d},
	//Timestamp:(*timestamp.Timestamp)(0xc00043d1a0),
	//IsDelete:false, XXX_NoUnkeyedLiteral:struct {}{},
	//XXX_unrecognized:[]uint8(nil),
	//XXX_sizecache:0}
	assetHistory, err := stub.GetHistoryForKey(assetID)
	if err != nil {
		return err
	}
	defer assetHistory.Close()
	for assetHistory.HasNext() {
		next, err := assetHistory.Next()
		if err != nil {
			return err
		}
		log.Printf("stub.GetHistoryForKey(%s), next=%#v", assetID, next)
	}

	return nil
}

func byteToString(data []byte) string {
	str := (*string)(unsafe.Pointer(&data))
	return *str
}

func (s *SmartContract) ContractPractice(ctx contractapi.TransactionContextInterface) {
	// s.GetName()是当前智能合约的名称，一个链码包中有多个智能合约，每个智能合约的名称必须不同，因此最好每个智能合约都实现这个方法来定义自己的名称
	log.Printf("s.GetName()=%s", s.GetName())
	log.Printf("s.GetInfo()=%#v", s.GetInfo())
	log.Printf("s.GetTransactionContextHandler()=%#v", s.GetTransactionContextHandler())
}

// ClientIdentityPractice ClientIdentity接口提供的方法练习
func (s *SmartContract) ClientIdentityPractice(ctx contractapi.TransactionContextInterface) error {
	log.Println("ClientIdentityPractice==================start=====================")
	clientIdentity := ctx.GetClientIdentity()
	id, err := clientIdentity.GetID()
	if err != nil {
		return err
	}
	log.Printf("clientIdentity.GetID()=%s", id)
	mspid, err := clientIdentity.GetMSPID()
	if err != nil {
		return err
	}
	log.Printf("clientIdentity.GetMSPID()=%s", mspid)
	certificate, err := clientIdentity.GetX509Certificate()
	if err != nil {
		return err
	}
	log.Printf("clientIdentity.GetX509Certificate()=%#v", certificate)
	value, found, err := clientIdentity.GetAttributeValue("test")
	if err != nil {
		return err
	}
	if found {
		log.Printf("clientIdentity.GetAttributeValue(\"test\")=%s", value)
	}

	if err := clientIdentity.AssertAttributeValue("test", "hello"); err != nil {
		log.Printf("clientIdentity.AssertAttributeValue(\"test\", \"hello\") error!")
		return err
	}

	log.Println("ClientIdentityPractice===================end======================")
	return nil
}

// GetUnknownTransaction returns the current set unknownTransaction, may be nil
func (s *SmartContract) GetUnknownTransaction() interface{} {
	return s.UnknownTransaction
}

// Default 如果不指定方法名称时指定的默认方法
func (s *SmartContract) UnknownTransaction(ctx contractapi.TransactionContextInterface) string {
	log.Printf("hello, i'm Default func！")
	return "Bye!"
}

// GetBeforeTransaction returns the current set beforeTransaction, may be nil
func (s *SmartContract) GetBeforeTransaction() interface{} {
	return s.BeforeTransaction
}

func (s *SmartContract) BeforeTransaction(ctx contractapi.TransactionContextInterface) {
	log.Printf("i'm BeforeTransaction")
}

// GetAfterTransaction returns the current set afterTransaction, may be nil
func (s *SmartContract) GetAfterTransaction() interface{} {
	return s.AfterTransaction
}

func (s *SmartContract) AfterTransaction(ctx contractapi.TransactionContextInterface) {
	log.Printf("i'm AfterTransaction")
}

func (s *SmartContract) IgnoredMe(ctx contractapi.TransactionContextInterface) {
	log.Printf("Ignored Me!")
}

func (s *SmartContract) GetIgnoredFunctions() []string {
	return []string{"IgnoredMe"}
}
