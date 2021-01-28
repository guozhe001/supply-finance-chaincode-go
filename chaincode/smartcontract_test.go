package chaincode

import (
	"encoding/json"
	"github.com/hyperledger/fabric-chaincode-go/pkg/statebased"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/stretchr/testify/require"
	"log"
	"testing"
)

func mockInitLedger(t *testing.T, stub *shimtest.MockStub) {
	assets := []Asset{
		{ID: AssetId, Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300},
		{ID: "asset2", Color: "red", Size: 5, Owner: "Brad", AppraisedValue: 400},
		{ID: "asset3", Color: "green", Size: 10, Owner: "Jin Soo", AppraisedValue: 500},
		{ID: "asset4", Color: "yellow", Size: 10, Owner: "Max", AppraisedValue: 600},
		{ID: "asset5", Color: "black", Size: 15, Owner: "Adriana", AppraisedValue: 700},
		{ID: "asset6", Color: "white", Size: 15, Owner: "Michel", AppraisedValue: 800},
	}
	stub.MockTransactionStart("test")
	putState(t, stub, assets...)
	id := stub.GetTxID()
	timestamp, err := stub.GetTxTimestamp()
	channelID := stub.GetChannelID()
	require.NoError(t, err)
	require.NotNil(t, timestamp)
	log.Printf("GetTxID()=%s, GetTxTimestamp()=%s, GetChannelID()=%s", id, timestamp, channelID)
	stub.MockTransactionEnd("test")
}

func marshal(asset Asset, t *testing.T) []byte {
	assetJSON, err := json.Marshal(asset)
	require.NoError(t, err)
	return assetJSON
}

// ChaincodeStubInterface#PutState()
func putState(t *testing.T, stub *shimtest.MockStub, assets ...Asset) {
	for _, asset := range assets {
		log.Printf("putState=%v", asset)
		require.NoError(t, stub.PutState(asset.ID, marshal(asset, t)))
	}
}

// ChaincodeStubInterface#GetState()
// ChaincodeStubInterface#PutState()
// ChaincodeStubInterface#DelState()
func getState(assetId string, t *testing.T, stub *shimtest.MockStub) {
	// 获取指定key的资产的世界状态
	state, err := stub.GetState(assetId)
	require.NoError(t, err)
	printAsset(t, state)
	newAssetID := "temp001"
	newAsset := Asset{ID: newAssetID, Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300}
	// put一个新的资产
	putState(t, stub, newAsset)
	// 查询新的资产
	newState, err := stub.GetState(newAssetID)
	require.NoError(t, err)
	require.NotNil(t, newState)
	printAsset(t, newState)
	// 指定资产ID删除资产
	require.NoError(t, stub.DelState(newAssetID))
	// 删除之后重新查询新的资产
	newStateAgain, err := stub.GetState(newAssetID)
	require.NoError(t, err)
	require.Nil(t, newStateAgain)
}

func getHistoryForKey(assetId string, t *testing.T, stub *shimtest.MockStub) {
	// 获取key的历史数据，目前mock还未实现
	history, err := stub.GetHistoryForKey(assetId)
	require.NoError(t, err)
	if history != nil {
		if history.HasNext() {
			next, err := history.Next()
			require.NoError(t, err)
			marshal, err := json.Marshal(next)
			require.NoError(t, err)
			log.Printf("asset=%s history=%s", assetId, marshal)
		}
		history.Close()
	}
}

func printAsset(t *testing.T, state []byte) {
	var a Asset
	require.NoError(t, json.Unmarshal(state, &a))
	marshal, err := json.Marshal(a)
	require.NoError(t, err)
	log.Printf("result state json value = %s", marshal)
}

// ChaincodeStubInterface#GetArgs()
// ChaincodeStubInterface#GetStringArgs()
func getArgs(t *testing.T, stub *shimtest.MockStub) {
	args := stub.GetArgs()
	for _, arg := range args {
		log.Printf("stub.GetArgs(), %s", byteToString(arg))
	}

	stringArgs := stub.GetStringArgs()
	for _, argString := range stringArgs {
		log.Print(argString)
	}
}

// ChaincodeStubInterface#GetStateByRange(startKey, endKey string) (StateQueryIteratorInterface, error)
// ChaincodeStubInterface#GetStateByRangeWithPagination(startKey, endKey string, pageSize int32, bookmark string) (StateQueryIteratorInterface, *pb.QueryResponseMetadata, error)
func getStateByRange(t *testing.T, stub *shimtest.MockStub) {
	// GetStateByRange不指定startKey和endKey，会返回全部的资产；谨慎使用
	states, err := stub.GetStateByRange("", "")
	require.NoError(t, err)
	printStateQueryIteratorInterface(t, states)
	// GetStateByRangeWithPagination 因为mockStub直接返回三个nil，所以无法在mock环境测试
	pagination, metadata, err := stub.GetStateByRangeWithPagination("", "", 5, "")
	require.NoError(t, err)
	log.Print("==========================================================================================")
	log.Printf("GetStateByRangeWithPagination metadata=%v", metadata)
	printStateQueryIteratorInterface(t, pagination)
}

func printStateQueryIteratorInterface(t *testing.T, states shim.StateQueryIteratorInterface) {
	if states != nil {
		for states.HasNext() {
			next, err := states.Next()
			require.NoError(t, err)
			log.Print(next)
		}
		states.Close()
	}
}

// ChaincodeStubInterface#CreateCompositeKey(objectType string, attributes []string) (string, error)
// ChaincodeStubInterface#SplitCompositeKey(compositeKey string) (string, []string, error)
func createCompositeKey(t *testing.T, stub *shimtest.MockStub) {
	objectType := "test"
	attributes := []string{"param1", "param2", "param3", "end"}
	// 创建组合key，拼接了一下
	key, err := stub.CreateCompositeKey(objectType, attributes)
	require.NoError(t, err)
	log.Printf("key=%s", key)
	// 分割组合key，CreateCompositeKey的逆运算
	compositeKey, strings, err := stub.SplitCompositeKey(key)
	require.Equal(t, objectType, compositeKey)
	require.Equal(t, attributes, strings)
	newAsset := Asset{ID: key, Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300}
	putState(t, stub, newAsset)
	empty := []string{}
	// 根据创建组合key的参数查询，后面的参数可以是空，这样会全部匹配出来
	states, err := stub.GetStateByPartialCompositeKey(objectType, empty)
	require.NoError(t, err)
	require.NotNil(t, states)
	printStateQueryIteratorInterface(t, states)
}

const (
	AssetId        string = "asset1"
	TestMSP        string = "TestMSP"
	TestCollection string = "private_TestMSP"
	Blank          string = ""
)

// ChaincodeStubInterface#SetStateValidationParameter(key string, ep []byte) error
// ChaincodeStubInterface#GetStateValidationParameter(key string) ([]byte, error)
func setStateValidationParameter(t *testing.T, stub *shimtest.MockStub) {
	// 新建一个基于状态的背书策略
	endorsementPolicy, err := statebased.NewStateEP(nil)
	require.NoError(t, err)
	// 向背书策略添加需要背书的公司
	require.NoError(t, endorsementPolicy.AddOrgs(statebased.RoleTypeMember, TestMSP))
	policy, err := endorsementPolicy.Policy()
	require.NoError(t, err)
	// SetStateValidationParameter设置基于状态的背书策略
	require.NoError(t, stub.SetStateValidationParameter(AssetId, policy))
	// GetStateValidationParameter获取基于状态的背书策略
	parameter, err := stub.GetStateValidationParameter(AssetId)
	require.NoError(t, err)
	str := byteToString(parameter)
	// 打印出来的StateValidationParameter有特殊字符，所以使用包含传入的字符的方式断言
	log.Printf("ID=%s, StateValidationParameter=%s", AssetId, str)
	require.Contains(t, str, TestMSP)
}

// ChaincodeStubInterface#GetPrivateData(collection, key string) ([]byte, error)
// ChaincodeStubInterface#GetPrivateDataHash(collection, key string) ([]byte, error) 获取私有数据的hash值，这个方法就算不是私有数据的所有者也可以调用，mock版本没有实现；
// ChaincodeStubInterface#DelPrivateData(collection, key string) error 删除私有数据，mock版本没有实现；
// ChaincodeStubInterface#SetPrivateDataValidationParameter(collection, key string, ep []byte) error 设置私有数据的
// ChaincodeStubInterface#GetPrivateDataValidationParameter(collection, key string) ([]byte, error)
// ChaincodeStubInterface#GetPrivateDataByRange(collection, startKey, endKey string) (StateQueryIteratorInterface, error) 根据范围查询私有数据
// ChaincodeStubInterface#GetPrivateDataByPartialCompositeKey(collection, objectType string, keys []string) (StateQueryIteratorInterface, error)
func getPrivateData(t *testing.T, stub *shimtest.MockStub) {
	key := "private001"
	privateAsset := Asset{ID: key, Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300}
	bytes, err := json.Marshal(privateAsset)
	require.NoError(t, err)
	// 添加私有数据
	require.NoError(t, stub.PutPrivateData(TestCollection, key, bytes))
	// 获取私有资产
	data, err := stub.GetPrivateData(TestCollection, key)
	require.NoError(t, err)
	require.NotNil(t, data)
	printAsset(t, data)
	// 使用不存在的其他的collection获取私有资产，不会返回error，会返回nil数据
	data, err = stub.GetPrivateData("test_collections", key)
	require.NoError(t, err)
	require.Nil(t, data)
	// 使用其他的key获取不存在私有资产
	data, err = stub.GetPrivateData(TestCollection, AssetId)
	require.NoError(t, err)
	require.Nil(t, data)
	// 查询公共资产数据,断言没有这个资产
	state, err := stub.GetState(key)
	require.NoError(t, err)
	require.Nil(t, state)

	endorsementPolicy, err := statebased.NewStateEP(nil)
	require.NoError(t, err)
	require.NoError(t, endorsementPolicy.AddOrgs(statebased.RoleTypeMember, TestMSP))
	policy, err := endorsementPolicy.Policy()
	require.NoError(t, err)
	require.NoError(t, stub.SetPrivateDataValidationParameter(TestCollection, key, policy))
	parameter, err := stub.GetPrivateDataValidationParameter(TestCollection, key)
	require.NoError(t, err)
	str := byteToString(parameter)
	// 打印出来的StateValidationParameter有特殊字符，所以使用包含传入的字符的方式断言
	log.Printf("ID=%s, StateValidationParameter=%s", AssetId, str)
	require.Contains(t, str, TestMSP)
	// GetPrivateDataHash(collection, key string) ([]byte, error) 获取私有数据的hash值，这个方法就算不是私有数据的所有者也可以调用，mock版本没有实现；
	// DelPrivateData(collection, key string) error 删除私有数据，mock版本没有实现；
	//require.NoError(t, stub.DelPrivateData(TestCollection, key))
	//// 删除之后再次查询，断言已经没有此资产
	//data, err = stub.GetPrivateData(TestCollection, key)
	//require.NoError(t, err)
	//require.Nil(t, state)
	// GetPrivateDataByRange没有实现
	//byRange, err := stub.GetPrivateDataByRange(TestCollection, Blank, Blank)
	//require.NoError(t, err)
	//require.NotNil(t, byRange)
	//for byRange.HasNext() {
	//	next, err := byRange.Next()
	//	require.NotNil(t, err)
	//	log.Print(next)
	//}
}

// ChaincodeStubInterface#ChaincodeStubInterface#GetCreator() ([]byte, error) 获取签约交易提议的人，签约提议的人也是这个交易的创建者; mockstub未实现
// ChaincodeStubInterface#GetTransient() (map[string][]byte, error) 获取临时数据，这个方法只有设置了临时数据的peer才能查到数据，主要是为了做隐私保护的，详情参考隐秘的交易资产
// ChaincodeStubInterface#GetBinding() ([]byte, error) TODO 待理解
// ChaincodeStubInterface#GetDecorations() ([]byte, error) TODO 待理解,目前看是为了传递更多关于提议的的额外数据
// ChaincodeStubInterface#GetSignedProposal() ([]byte, error) 获取提议; mockstub未实现
// ChaincodeStubInterface#SetEvent(name string, payload []byte) error  允许链码在提议的response设置一个事件。无论交易的有效性如何，事件都将在已提交的块中的交易内可用。一个交易只能包含一个事件，并且如果是链码调用另一个链码的情况，事件只能在最外层。
func stubOthers(t *testing.T, stub *shimtest.MockStub) {
	m := make(map[string][]byte)
	tempAsset := Asset{ID: "temp001", Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300}
	m["temp_asset"], _ = json.Marshal(tempAsset)
	require.NoError(t, stub.SetTransient(m))
	transient, err := stub.GetTransient()
	require.NoError(t, err)
	require.NotNil(t, transient)
	for k, v := range transient {
		log.Printf("k=%s, v=%s", k, v)
	}
	decorations := stub.GetDecorations()
	for k, v := range decorations {
		log.Printf("k=%s, v=%s", k, v)
	}
}

// 测试shim.ChaincodeStubInterface接口
func stubTest(t *testing.T, stub *shimtest.MockStub) {
	assetId := AssetId
	mockInitLedger(t, stub)
	stub.MockTransactionStart("test1")
	getState(assetId, t, stub)
	//getHistoryForKey(assetId, t, stub)
	getArgs(t, stub)
	stub.MockTransactionStart("test1")
	getStateByRange(t, stub)
	createCompositeKey(t, stub)
	setStateValidationParameter(t, stub)
	getPrivateData(t, stub)
	stubOthers(t, stub)
}

// 测试contractapi.Contract的方法
func contractTest(t *testing.T, ccc *contractapi.ContractChaincode, stub *shimtest.MockStub) {
	log.Printf("DefaultContract=%s", ccc.DefaultContract)
	info := ccc.Info
	log.Printf("info=%v", info)
	stub.MockTransactionStart("contract_test")
	// 如果调用一个不存在的方法，如果实现了GetUnknownTransaction接口，则会执行此接口返回的方法；否则不执行，并且也不会报错，但是如果有before方法是会执行的
	response := stub.MockInvoke("uuid_002", [][]byte{[]byte("Unknow")})
	log.Printf("response=%#v, response.Status=%d, response.Payload=%s", response, response.Status, byteToString(response.Payload))
	// 调用一个被忽略的方法, 虽然IgnoredMe方法在智能合约中存在，但是因为合约满足IgnoreContractInterface接口然后把这个方法加入到了忽略列表中，所以最后还是调用的默认方法
	response = stub.MockInvoke("uuid_002", [][]byte{[]byte("IgnoredMe")})
	log.Printf("response=%#v, response.Status=%d, response.Payload=%s", response, response.Status, byteToString(response.Payload))
	// 指定某个指定合约，调用一个不存在的方法，冒号前面的部分是智能合约名称，后面是方法名称
	response = stub.MockInvoke("uuid_002", [][]byte{[]byte("TestSmartContract:Unknow")})
	log.Printf("response=%#v, response.Status=%d, response.Payload=%s", response, response.Status, byteToString(response.Payload))
	//invoke := ccc.Invoke(stub)
	//log.Printf("response=%v", invoke)
	stub.MockTransactionEnd("uuid_001")
	transactionSerializer := ccc.TransactionSerializer
	log.Printf("transactionSerializer=%v", transactionSerializer)
}

// 测试入口
func TestStart(t *testing.T) {
	// 一个链码包中可以有多个智能合约
	assetChaincode, err := contractapi.NewChaincode(&SmartContract{}, &TestSmartContract{})
	require.NoError(t, err)
	// NewMockStub
	stub := shimtest.NewMockStub("mockSub", assetChaincode)
	stubTest(t, stub)
	contractTest(t, assetChaincode, stub)
}

type TestSmartContract struct {
	contractapi.Contract
}

// GetUnknownTransaction returns the current set unknownTransaction, may be nil
func (t *TestSmartContract) GetUnknownTransaction() interface{} {
	return t.UnknownTransaction
}

// Default 如果不指定方法名称时指定的默认方法
func (t *TestSmartContract) UnknownTransaction(ctx contractapi.TransactionContextInterface) string {
	log.Printf("hello, i'm Default func in TestSmartContract！")
	return "i'm TestSmartContract, Bye!"
}
