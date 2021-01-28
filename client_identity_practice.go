package main

import (
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"log"
)

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
