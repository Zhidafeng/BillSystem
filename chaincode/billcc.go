package main

import (
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"encoding/json"
	"github.com/hyperledger/fabric/protos/peer"
)

// 保存票据对象至账本中
func (t *BillChainCode) pubBill(stub shim.ChaincodeStubInterface, bill Bill) ([]byte, bool) {

	b, err := json.Marshal(bill)
	if err != nil {
		return nil, false
	}

	err = stub.PutState(Bill_Prefix + bill.BillInfoID, b)
	if err != nil {
		return nil, false
	}

	return b, true
}

// 根据指定的票据号码查询相应的票据对象
func (t *BillChainCode) getBill(stub shim.ChaincodeStubInterface, billNo string) (Bill, bool)  {
	var bill Bill

	b, err := stub.GetState(Bill_Prefix + billNo)
	if err != nil {
		return bill, false
	}

	err = json.Unmarshal(b, &bill)
	if err != nil {
		return bill, false
	}

	return bill, true

}

// 发布票据
// args: bill
func (t *BillChainCode) issue(stub shim.ChaincodeStubInterface, args []string) peer.Response  {
	if len(args) != 1 {
		return shim.Error("发布票据失败, 指定的票据内容错误")
	}

	var bill Bill
	err := json.Unmarshal([]byte(args[0]), &bill)
	if err != nil {
		return shim.Error("反序列票据对象时发生错误")
	}

	_, bl := t.getBill(stub, bill.BillInfoID)
	if bl {
		return shim.Error("发布的票据已存在")
	}

	bill.State = BillInfo_State_NewPublish

	_, bl = t.pubBill(stub, bill)
	if !bl {
		return shim.Error("保存票据信息时发生错误")
	}

	// 根据当前持票人ID与票据号码定义复合Key, 方便后期批量查询
	holderCmIDBillInfoIDIndexKey, err := stub.CreateCompositeKey(IndexName, []string{bill.HoldrCmID, bill.BillInfoID})
	if err != nil{
		return shim.Error("创建复合键时发生错误")
	}
	err = stub.PutState(holderCmIDBillInfoIDIndexKey, []byte{0x00})
	if err != nil {
		return shim.Error("保存复合键时发生错误")
	}

	return shim.Success([]byte("指定的票据发布成功"))
}

// 根据当前持票人证件号码批量查询所持票据
// args: holderCmID
func (t *BillChainCode) queryMyBills(stub shim.ChaincodeStubInterface, args []string) peer.Response  {
	if len(args) != 1 {
		return shim.Error("必须且只能指定当前用户的证件号码")
	}

	iterator, err := stub.GetStateByPartialCompositeKey(IndexName, []string{args[0]})
	if err != nil {
		return shim.Error("根据指定持票人证件号码查询信息时发生错误 ")
	}
	defer iterator.Close()

	var bills []Bill
	for iterator.HasNext() {
		kv, _ := iterator.Next()
		_, compositeKey, err := stub.SplitCompositeKey(kv.Key)
		if err != nil {
			return shim.Error("分割指定的复合键时发生错误")
		}

		bill, bl := t.getBill(stub, compositeKey[1])
		if !bl{
			return shim.Error("根据指定的票据号码查询票据信息时发生错误")
		}
		bills = append(bills, bill)
	}

	bs, err := json.Marshal(bills)
	if err != nil {
		return shim.Error("序列化票据时发生错误")
	}

	return shim.Success(bs)

}

// 根据指定的票据号码查询该票据的详情
// args: billInfoID
func (t *BillChainCode) queryBillByNo(stub shim.ChaincodeStubInterface, args []string) peer.Response  {
	if len(args) != 1{
		return  shim.Error("必须且只能指定要查询的票据号码")
	}

	bill, bl := t.getBill(stub, args[0])
	if !bl{
		return shim.Error("根据指定的票据号码查询对应信息时失败")
	}

	iterator, err := stub.GetHistoryForKey(Bill_Prefix + bill.BillInfoID)
	if err != nil{
		return shim.Error("根据指定的票据号码查询历史流转信息时失败")
	}
	defer iterator.Close()

	var bills []HistoryItem
	var hisBill Bill
	for iterator.HasNext()  {
		hisData, err := iterator.Next()
		if err != nil {
			return shim.Error("获取历史流转信息时发生错误")
		}

		var historyItem HistoryItem
		historyItem.TxId = hisData.TxId
		json.Unmarshal(hisData.Value, &hisBill)
		if hisData.Value == nil {
			var empty Bill
			historyItem.Bill = empty
		}else {
			historyItem.Bill = hisBill
		}

		bills = append(bills, historyItem)
	}


	bill.History = bills

	b, err := json.Marshal(bill)
	if err != nil {
		return shim.Error("序列化票据时发生错误")
	}
	return shim.Success(b)
}

// 查询当前用户的待背书票据
// args: WaitEndorseCmID
func (t *BillChainCode) queryMyWaitBills(stub shim.ChaincodeStubInterface, args []string) peer.Response  {
	if len(args) != 1 {
		return shim.Error("必须且只能指定待背书人证件号码")
	}

	iterator, err := stub.GetStateByPartialCompositeKey(IndexName, []string{args[0]})
	if err != nil {
		return shim.Error("根据指定待背书人证件号码查询复合键时发生错误")
	}
	defer iterator.Close()

	var bills []Bill
	for iterator.HasNext() {
		kv, _ := iterator.Next()
		_, composite, err := stub.SplitCompositeKey(kv.Key)
		if err != nil {
			return shim.Error("分割复合key时发生错误")
		}
		bill, bl := t.getBill(stub, composite[1])
		if !bl{
			return shim.Error("根据指定的票据号码查询票据信息时发生错误")
		}

		if bill.State == BillInfo_State_EndorseWaitSign && bill.WaitEndorseCmID == args[0]{
			bills = append(bills, bill)
		}

	}

	b, err := json.Marshal(bills)
	if err != nil {
		return shim.Error("序列化待背书票据时发生错误")
	}
	return shim.Success(b)

}