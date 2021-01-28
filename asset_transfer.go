/*
 SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/guozhe001/supply-finance-chaincode-go/chaincode"
	"log"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/hyperledger/fabric-chaincode-go/pkg/statebased"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

const (
	typeAssetForSale     = "S"
	typeAssetBid         = "B"
	typeAssetSaleReceipt = "SR"
	typeAssetBuyReceipt  = "BR"
	statusEnable         = "enable"
	statusDelete         = "delete"
)

type SmartContract struct {
	contractapi.Contract
}

// Asset struct and properties must be exported (start with capitals) to work with contract api metadata
type Asset struct {
	ObjectType        string `json:"objectType"` // ObjectType is used to distinguish different object types in the same chaincode namespace
	ID                string `json:"assetID"`
	OwnerOrg          string `json:"ownerOrg"`
	PublicDescription string `json:"publicDescription"`
	Status            string `json:"status"`
	ParentID          string `json:"parentID"`
}

type receipt struct {
	price     int
	timestamp time.Time
}

// AssetProperties 资产属性
type AssetProperties struct {
	ObjectType string    `json:"objectType"` // ObjectType is used to distinguish different object types in the same chaincode namespace
	ID         string    `json:"assetID"`
	Issuer     string    `json:"issuer"`
	Amount     int       `json:"amount"`
	CreateDate time.Time `json:"createDate"`
	EndDate    time.Time `json:"endDate"`
	Salt       string    `json:"salt"`
}

// CreateAsset creates an asset and sets it as owned by the client's org
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, assetID, publicDescription string) error {
	// 获取临时数据库的数据，返回一个map[string][]byte
	transientMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient: %v", err)
	}

	// Asset properties must be retrieved from the transient field as they are private
	immutablePropertiesJSON, ok := transientMap["asset_properties"]
	fmt.Println("immutablePropertiesJSON:", immutablePropertiesJSON)
	if !ok {
		return fmt.Errorf("asset_properties key not found in the transient map")
	}

	return createAsset(ctx, immutablePropertiesJSON, assetID, publicDescription, "")
}

// CreateAsset creates an asset and sets it as owned by the client's org
func createAsset(ctx contractapi.TransactionContextInterface, immutablePropertiesJSON []byte, assetID, publicDescription string,
	parentID string) error {
	// Get client org id and verify it matches peer org id.
	// In this scenario, client is only authorized to read/write private data from its own peer.
	clientOrgID, err := getClientOrgID(ctx, true)
	fmt.Println("clientOrgID:", clientOrgID)
	if err != nil {
		return fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	asset := Asset{
		ObjectType:        "asset",
		ID:                assetID,
		OwnerOrg:          clientOrgID,
		PublicDescription: publicDescription,
		Status:            statusEnable,
		ParentID:          parentID,
	}
	fmt.Println("asset:", asset)
	assetBytes, err := json.Marshal(asset)
	if err != nil {
		return fmt.Errorf("failed to create asset JSON: %v", err)
	}

	err = ctx.GetStub().PutState(asset.ID, assetBytes)
	if err != nil {
		return fmt.Errorf("failed to put asset in public data: %v", err)
	}

	// Set the endorsement policy such that an owner org peer is required to endorse future updates
	err = setAssetStateBasedEndorsement(ctx, asset.ID, clientOrgID)
	if err != nil {
		return fmt.Errorf("failed setting state based endorsement for owner: %v", err)
	}

	// Persist private immutable asset properties to owner's private data collection
	collection := buildCollectionName(clientOrgID)
	fmt.Println("collection:", collection)
	err = ctx.GetStub().PutPrivateData(collection, asset.ID, immutablePropertiesJSON)
	if err != nil {
		return fmt.Errorf("failed to put Asset private details: %v", err)
	}
	return nil
}

// // verifyAssetProperties 验证资产属性的信息
// func verifyAssetProperties(immutablePropertiesJSON []byte, asset Asset) error {
// 	assetProperties, err := getAssetProperties(immutablePropertiesJSON)
// 	if err != nil {
// 		return err
// 	}
// 	// 资产的属性ID和资产ID相同
// 	if asset.ID != assetProperties.ID {
// 		return fmt.Errorf("资产ID和资产属性ID必须相同")
// 	}
// 	// 资产的发行者就是资产的创建者，所有人都可以发行，但是别人认不认可这个组织发行的资产是另一回事
// 	if asset.OwnerOrg != assetProperties.Issuer {
// 		return fmt.Errorf("资产的发行方必须是当前创建资产的组织")
// 	}
// 	// 理论上这里应该还有更多的校验，比如说创建时间和失效时间的验证
// 	return nil
// }
// ChangePublicDescription updates the assets public description. Only the current owner can update the public description
func (s *SmartContract) ChangePublicDescription(ctx contractapi.TransactionContextInterface, assetID string, newDescription string) error {
	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %v", err)
	}
	return changeOriginAssetInfo(ctx, *asset, "", newDescription)
}

// AgreeToSell adds seller's asking price to seller's implicit private data collection
func (s *SmartContract) AgreeToSell(ctx contractapi.TransactionContextInterface, assetID string) error {
	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return err
	}

	clientOrgID, err := getClientOrgID(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	// Verify that this clientOrgId actually owns the asset.
	if clientOrgID != asset.OwnerOrg {
		return fmt.Errorf("a client from %s cannot sell an asset owned by %s", clientOrgID, asset.OwnerOrg)
	}

	return agreeToPrice(ctx, assetID, typeAssetForSale)
}

// AgreeToBuy adds buyer's bid price to buyer's implicit private data collection
func (s *SmartContract) AgreeToBuy(ctx contractapi.TransactionContextInterface, assetID string) error {
	return agreeToPrice(ctx, assetID, typeAssetBid)
}

// agreeToPrice adds a bid or ask price to caller's implicit private data collection
func agreeToPrice(ctx contractapi.TransactionContextInterface, assetID string, priceType string) error {
	// In this scenario, client is only authorized to read/write private data from its own peer.
	clientOrgID, err := getClientOrgID(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	transMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient: %v", err)
	}

	// Asset price must be retrieved from the transient field as they are private
	price, ok := transMap["asset_price"]
	if !ok {
		return fmt.Errorf("asset_price key not found in the transient map")
	}

	collection := buildCollectionName(clientOrgID)

	// Persist the agreed to price in a collection sub-namespace based on priceType key prefix,
	// to avoid collisions between private asset properties, sell price, and buy price
	assetPriceKey, err := ctx.GetStub().CreateCompositeKey(priceType, []string{assetID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	// The Price hash will be verified later, therefore always pass and persist price bytes as is,
	// so that there is no risk of nondeterministic marshaling.
	err = ctx.GetStub().PutPrivateData(collection, assetPriceKey, price)
	if err != nil {
		return fmt.Errorf("failed to put asset bid: %v", err)
	}

	return nil
}

// VerifyAssetProperties  Allows a buyer to validate the properties of
// an asset against the owner's implicit private data collection
func (s *SmartContract) VerifyAssetProperties(ctx contractapi.TransactionContextInterface, assetID string) (bool, error) {
	transMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return false, fmt.Errorf("error getting transient: %v", err)
	}

	/// Asset properties must be retrieved from the transient field as they are private
	immutablePropertiesJSON, ok := transMap["asset_properties"]
	if !ok {
		return false, fmt.Errorf("asset_properties key not found in the transient map")
	}

	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return false, fmt.Errorf("failed to get asset: %v", err)
	}

	// 添加资产状态的验证
	if (*asset).Status != statusEnable {
		return false, fmt.Errorf("资产不可以，不允许交易: %v", err)
	}

	collectionOwner := buildCollectionName(asset.OwnerOrg)
	immutablePropertiesOnChainHash, err := ctx.GetStub().GetPrivateDataHash(collectionOwner, assetID)
	if err != nil {
		return false, fmt.Errorf("failed to read asset private properties hash from seller's collection: %v", err)
	}
	if immutablePropertiesOnChainHash == nil {
		return false, fmt.Errorf("asset private properties hash does not exist: %s", assetID)
	}

	hash := sha256.New()
	hash.Write(immutablePropertiesJSON)
	calculatedPropertiesHash := hash.Sum(nil)

	// verify that the hash of the passed immutable properties matches the on-chain hash
	if !bytes.Equal(immutablePropertiesOnChainHash, calculatedPropertiesHash) {
		return false, fmt.Errorf("hash %x for passed immutable properties %s does not match on-chain hash %x",
			calculatedPropertiesHash,
			immutablePropertiesJSON,
			immutablePropertiesOnChainHash,
		)
	}

	return true, nil
}

// TransferAsset checks transfer conditions and then transfers asset state to buyer.
// TransferAsset can only be called by current owner
func (s *SmartContract) TransferAsset(ctx contractapi.TransactionContextInterface, assetID string, buyerOrgID string) error {
	clientOrgID, err := getClientOrgID(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	transMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient data: %v", err)
	}

	immutablePropertiesJSON, ok := transMap["asset_properties"]
	if !ok {
		return fmt.Errorf("asset_properties key not found in the transient map")
	}

	priceJSON, ok := transMap["asset_price"]
	if !ok {
		return fmt.Errorf("asset_price key not found in the transient map")
	}

	var agreement Agreement
	err = json.Unmarshal(priceJSON, &agreement)
	if err != nil {
		return fmt.Errorf("failed to unmarshal price JSON: %v", err)
	}

	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %v", err)
	}

	// 添加资产状态的验证
	if (*asset).Status != statusEnable {
		return fmt.Errorf("资产不可以，不允许交易")
	}

	err = verifyTransferConditions(ctx, asset, immutablePropertiesJSON, clientOrgID, buyerOrgID, priceJSON)
	if err != nil {
		return fmt.Errorf("failed transfer verification: %v", err)
	}

	err = transferAssetState(ctx, asset, immutablePropertiesJSON, clientOrgID, buyerOrgID, agreement.Price)
	if err != nil {
		return fmt.Errorf("failed asset transfer: %v", err)
	}

	return nil

}

// SplitAsset 拆分资产为两个资产，传入的amount是拆分后的其中一个资产的金额
func (s *SmartContract) SplitAsset(ctx contractapi.TransactionContextInterface, assetID string, amount int) error {
	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return err
	}
	immutableProperties, err := getAssetPrivateProperties(ctx, assetID)
	if err != nil {
		return err
	}
	assetProperties, err := getAssetProperties(immutableProperties)
	if err != nil {
		return err
	}
	if assetProperties.Amount <= amount {
		return fmt.Errorf("资产ID的金额为%d小于想要拆分的金额为%d，不允许拆分", assetProperties.Amount, amount)
	}
	if err := splitAsset(ctx, assetProperties, assetID+"1", amount, *asset); err != nil {
		return err
	}
	if err := splitAsset(ctx, assetProperties, assetID+"2", assetProperties.Amount-amount, *asset); err != nil {
		return err
	}
	// 拆分之后删除旧资产
	collection := buildCollectionName((*asset).OwnerOrg)
	err = ctx.GetStub().DelPrivateData(collection, asset.ID)
	if err != nil {
		return fmt.Errorf("failed to delete Asset private details from org: %v", err)
	}
	// 修改公共资产信息
	changeOriginAssetInfo(ctx, *asset, statusDelete, "已拆分")
	return nil
}

// 根据transient获取的assetProperties的字节数组获取AssetProperties
func getAssetProperties(immutablePropertiesJSON []byte) (AssetProperties, error) {
	var assetProperties AssetProperties
	if err := json.Unmarshal(immutablePropertiesJSON, &assetProperties); err != nil {
		return assetProperties, fmt.Errorf("failed to unmarshal price JSON: %v", err)
	}
	return assetProperties, nil
}

// ChangePublicDescription updates the assets public description. Only the current owner can update the public description
func changeOriginAssetInfo(ctx contractapi.TransactionContextInterface, asset Asset, status string, newDescription string) error {
	// No need to check client org id matches peer org id, rely on the asset ownership check instead.
	clientOrgID, err := getClientOrgID(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	// Auth check to ensure that client's org actually owns the asset
	if clientOrgID != asset.OwnerOrg {
		return fmt.Errorf("a client from %s cannot update the description of a asset owned by %s", clientOrgID, asset.OwnerOrg)
	}

	// 添加资产状态的验证
	if asset.Status != statusEnable {
		return fmt.Errorf("资产不可用，不允许修改")
	}
	if status != "" {
		asset.Status = status
	}
	if newDescription != "" {
		asset.PublicDescription = newDescription
	}
	updatedAssetJSON, err := json.Marshal(asset)
	if err != nil {
		return fmt.Errorf("failed to marshal asset: %v", err)
	}

	return ctx.GetStub().PutState(asset.ID, updatedAssetJSON)
}

// splitAsset 从原始资产属性拆分成指定ID和金额的资产
func splitAsset(ctx contractapi.TransactionContextInterface, originAssetProperties AssetProperties, newAssetID string, newAmount int,
	asset Asset) error {
	originAssetProperties.Amount = newAmount
	originAssetProperties.ID = newAssetID
	immutablePropertiesJSON, err := json.Marshal(originAssetProperties)
	if err != nil {
		return err
	}
	return createAsset(ctx, immutablePropertiesJSON, newAssetID, asset.PublicDescription, asset.ID)
}

// verifyTransferConditions checks that client org currently owns asset and that both parties have agreed on price
func verifyTransferConditions(ctx contractapi.TransactionContextInterface,
	asset *Asset,
	immutablePropertiesJSON []byte,
	clientOrgID string,
	buyerOrgID string,
	priceJSON []byte) error {

	// CHECK1: Auth check to ensure that client's org actually owns the asset

	if clientOrgID != asset.OwnerOrg {
		return fmt.Errorf("a client from %s cannot transfer a asset owned by %s", clientOrgID, asset.OwnerOrg)
	}

	// CHECK2: Verify that the hash of the passed immutable properties matches the on-chain hash

	collectionSeller := buildCollectionName(clientOrgID)
	immutablePropertiesOnChainHash, err := ctx.GetStub().GetPrivateDataHash(collectionSeller, asset.ID)
	if err != nil {
		return fmt.Errorf("failed to read asset private properties hash from seller's collection: %v", err)
	}
	if immutablePropertiesOnChainHash == nil {
		return fmt.Errorf("asset private properties hash does not exist: %s", asset.ID)
	}

	hash := sha256.New()
	hash.Write(immutablePropertiesJSON)
	calculatedPropertiesHash := hash.Sum(nil)

	// verify that the hash of the passed immutable properties matches the on-chain hash
	if !bytes.Equal(immutablePropertiesOnChainHash, calculatedPropertiesHash) {
		return fmt.Errorf("hash %x for passed immutable properties %s does not match on-chain hash %x",
			calculatedPropertiesHash,
			immutablePropertiesJSON,
			immutablePropertiesOnChainHash,
		)
	}

	// CHECK3: Verify that seller and buyer agreed on the same price

	// Get sellers asking price
	assetForSaleKey, err := ctx.GetStub().CreateCompositeKey(typeAssetForSale, []string{asset.ID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}
	sellerPriceHash, err := ctx.GetStub().GetPrivateDataHash(collectionSeller, assetForSaleKey)
	if err != nil {
		return fmt.Errorf("failed to get seller price hash: %v", err)
	}
	if sellerPriceHash == nil {
		return fmt.Errorf("seller price for %s does not exist", asset.ID)
	}

	// Get buyers bid price
	collectionBuyer := buildCollectionName(buyerOrgID)
	assetBidKey, err := ctx.GetStub().CreateCompositeKey(typeAssetBid, []string{asset.ID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}
	// TODO 疑问：这个方法是由资产拥有者调用的，那么资产拥有者怎么可以获取资产买方的出价信息呢？如果是从公共状态获取购买方的出价hash是没问题的，但是从购买方的私有数据集中获取出价hash很让人费解。
	buyerPriceHash, err := ctx.GetStub().GetPrivateDataHash(collectionBuyer, assetBidKey)
	if err != nil {
		return fmt.Errorf("failed to get buyer price hash: %v", err)
	}
	if buyerPriceHash == nil {
		return fmt.Errorf("buyer price for %s does not exist", asset.ID)
	}

	hash = sha256.New()
	hash.Write(priceJSON)
	calculatedPriceHash := hash.Sum(nil)

	// Verify that the hash of the passed price matches the on-chain sellers price hash
	if !bytes.Equal(calculatedPriceHash, sellerPriceHash) {
		return fmt.Errorf("hash %x for passed price JSON %s does not match on-chain hash %x, seller hasn't agreed to the passed trade id and price",
			calculatedPriceHash,
			priceJSON,
			sellerPriceHash,
		)
	}

	// Verify that the hash of the passed price matches the on-chain buyer price hash
	if !bytes.Equal(calculatedPriceHash, buyerPriceHash) {
		return fmt.Errorf("hash %x for passed price JSON %s does not match on-chain hash %x, buyer hasn't agreed to the passed trade id and price",
			calculatedPriceHash,
			priceJSON,
			buyerPriceHash,
		)
	}

	return nil
}

// transferAssetState performs the public and private state updates for the transferred asset
func transferAssetState(ctx contractapi.TransactionContextInterface, asset *Asset, immutablePropertiesJSON []byte, clientOrgID string, buyerOrgID string, price int) error {
	asset.OwnerOrg = buyerOrgID
	updatedAsset, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(asset.ID, updatedAsset)
	if err != nil {
		return fmt.Errorf("failed to write asset for buyer: %v", err)
	}

	// Change the endorsement policy to the new owner
	err = setAssetStateBasedEndorsement(ctx, asset.ID, buyerOrgID)
	if err != nil {
		return fmt.Errorf("failed setting state based endorsement for new owner: %v", err)
	}

	// Transfer the private properties (delete from seller collection, create in buyer collection)
	collectionSeller := buildCollectionName(clientOrgID)
	err = ctx.GetStub().DelPrivateData(collectionSeller, asset.ID)
	if err != nil {
		return fmt.Errorf("failed to delete Asset private details from seller: %v", err)
	}

	collectionBuyer := buildCollectionName(buyerOrgID)
	err = ctx.GetStub().PutPrivateData(collectionBuyer, asset.ID, immutablePropertiesJSON)
	if err != nil {
		return fmt.Errorf("failed to put Asset private properties for buyer: %v", err)
	}

	// Delete the price records for seller
	assetPriceKey, err := ctx.GetStub().CreateCompositeKey(typeAssetForSale, []string{asset.ID})
	if err != nil {
		return fmt.Errorf("failed to create composite key for seller: %v", err)
	}

	err = ctx.GetStub().DelPrivateData(collectionSeller, assetPriceKey)
	if err != nil {
		return fmt.Errorf("failed to delete asset price from implicit private data collection for seller: %v", err)
	}

	// Delete the price records for buyer
	assetPriceKey, err = ctx.GetStub().CreateCompositeKey(typeAssetBid, []string{asset.ID})
	if err != nil {
		return fmt.Errorf("failed to create composite key for buyer: %v", err)
	}

	err = ctx.GetStub().DelPrivateData(collectionBuyer, assetPriceKey)
	if err != nil {
		return fmt.Errorf("failed to delete asset price from implicit private data collection for buyer: %v", err)
	}

	// Keep record for a 'receipt' in both buyers and sellers private data collection to record the sale price and date.
	// Persist the agreed to price in a collection sub-namespace based on receipt key prefix.
	receiptBuyKey, err := ctx.GetStub().CreateCompositeKey(typeAssetBuyReceipt, []string{asset.ID, ctx.GetStub().GetTxID()})
	if err != nil {
		return fmt.Errorf("failed to create composite key for receipt: %v", err)
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to create timestamp for receipt: %v", err)
	}

	timestamp, err := ptypes.Timestamp(txTimestamp)
	if err != nil {
		return err
	}
	assetReceipt := receipt{
		price:     price,
		timestamp: timestamp,
	}
	receipt, err := json.Marshal(assetReceipt)
	if err != nil {
		return fmt.Errorf("failed to marshal receipt: %v", err)
	}

	err = ctx.GetStub().PutPrivateData(collectionBuyer, receiptBuyKey, receipt)
	if err != nil {
		return fmt.Errorf("failed to put private asset receipt for buyer: %v", err)
	}

	receiptSaleKey, err := ctx.GetStub().CreateCompositeKey(typeAssetSaleReceipt, []string{ctx.GetStub().GetTxID(), asset.ID})
	if err != nil {
		return fmt.Errorf("failed to create composite key for receipt: %v", err)
	}

	err = ctx.GetStub().PutPrivateData(collectionSeller, receiptSaleKey, receipt)
	if err != nil {
		return fmt.Errorf("failed to put private asset receipt for seller: %v", err)
	}

	return nil
}

// getClientOrgID gets the client org ID.
// The client org ID can optionally be verified against the peer org ID, to ensure that a client
// from another org doesn't attempt to read or write private data from this peer.
// The only exception in this scenario is for TransferAsset, since the current owner
// needs to get an endorsement from the buyer's peer.
func getClientOrgID(ctx contractapi.TransactionContextInterface, verifyOrg bool) (string, error) {
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("failed getting client's orgID: %v", err)
	}

	if verifyOrg {
		err = verifyClientOrgMatchesPeerOrg(clientOrgID)
		if err != nil {
			return "", err
		}
	}

	return clientOrgID, nil
}

// verifyClientOrgMatchesPeerOrg checks the client org id matches the peer org id.
func verifyClientOrgMatchesPeerOrg(clientOrgID string) error {
	peerOrgID, err := shim.GetMSPID()
	if err != nil {
		return fmt.Errorf("failed getting peer's orgID: %v", err)
	}

	if clientOrgID != peerOrgID {
		return fmt.Errorf("client from org %s is not authorized to read or write private data from an org %s peer",
			clientOrgID,
			peerOrgID,
		)
	}

	return nil
}

// setAssetStateBasedEndorsement adds an endorsement policy to a asset so that only a peer from an owning org
// can update or transfer the asset.
func setAssetStateBasedEndorsement(ctx contractapi.TransactionContextInterface, assetID string, orgToEndorse string) error {
	endorsementPolicy, err := statebased.NewStateEP(nil)
	if err != nil {
		return err
	}
	err = endorsementPolicy.AddOrgs(statebased.RoleTypeMember, orgToEndorse)
	if err != nil {
		return fmt.Errorf("failed to add org to endorsement policy: %v", err)
	}
	policy, err := endorsementPolicy.Policy()
	if err != nil {
		return fmt.Errorf("failed to create endorsement policy bytes from org: %v", err)
	}
	// fmt.Printf("assetID=%s, orgToEndorse=%s, policy=%s, len(policy)=%d \n", assetID, orgToEndorse, policy, len(policy))
	// fmt.Printf("assetID=%s, policy=%s, endorsementPolicy.ListOrgs=%s\n", assetID, string(policy[:]), endorsementPolicy.ListOrgs())
	return ctx.GetStub().SetStateValidationParameter(assetID, policy)
}

func buildCollectionName(clientOrgID string) string {
	return fmt.Sprintf("_implicit_org_%s", clientOrgID)
}

func getClientImplicitCollectionName(ctx contractapi.TransactionContextInterface) (string, error) {
	clientOrgID, err := getClientOrgID(ctx, true)
	if err != nil {
		return "", fmt.Errorf("failed to get verified OrgID: %v", err)
	}

	err = verifyClientOrgMatchesPeerOrg(clientOrgID)
	if err != nil {
		return "", err
	}

	return buildCollectionName(clientOrgID), nil
}

func main() {
	ccc, err := contractapi.NewChaincode(new(SmartContract), new(chaincode.SmartContract))
	if err != nil {
		log.Panicf("Error create transfer asset chaincode: %v", err)
	}

	if err := ccc.Start(); err != nil {
		log.Panicf("Error starting asset chaincode: %v", err)
	}
}
