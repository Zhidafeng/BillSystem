package blockchain

import (
	"github.com/hyperledger/fabric-sdk-go/api/apitxn/resmgmtclient"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/config"
	"github.com/hyperledger/fabric-sdk-go/api/apitxn/chmgmtclient"
	"time"
	"github.com/hyperledger/fabric-sdk-go/api/apitxn/chclient"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabric-client/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
)

const Chaincode_Version  = "1.0"

type FabricSetup struct {
	ConfigFile		string		// SDK所需参数的配置文件路径
	ChannelID		string		// 通道名称
	Initialize		bool		// 是否已被初始化标识
	OrgAdmin		string		// 组织管理员名称
	OrgName			string		// 组织名称
	ChannelConfig	string		// 应用通道交易配置文件所在路径

	admin			resmgmtclient.ResourceMgmtClient
	sdk				*fabsdk.FabricSDK


	ChaincodeID		string	// 链码名称
	ChaincodeGOPath	string	// 系统GOPATH环境变量
	ChaincodePath	string	// 链码文件所在路径

	UserName		string	// 操作用户名称
	Client			chclient.ChannelClient	// 具有操作权限的客户端对象
}

// 创建并初始化Fabric-SDK
func (t *FabricSetup) Initialized() error {
	fmt.Println("开始初始化SDK...")

	if t.Initialize {
		return fmt.Errorf("SDK已被实例化!")
	}

	sdk, err := fabsdk.New(config.FromFile(t.ConfigFile))
	if err != nil {
		return fmt.Errorf("SDK创建失败: %s", err)
	}

	t.sdk = sdk

	chMgmtClient, err := t.sdk.NewClient(fabsdk.WithUser(t.OrgAdmin), fabsdk.WithOrg(t.OrgName)).ChannelMgmt()
	if err != nil {
		return fmt.Errorf("创建应用通道管理对象失败: %s", err)
	}

	session, err := t.sdk.NewClient(fabsdk.WithUser(t.OrgAdmin), fabsdk.WithOrg(t.OrgName)).Session()
	if err != nil {
		return fmt.Errorf("获取会话用户失败: %s", err)
	}

	req := chmgmtclient.SaveChannelRequest{ChannelID:t.ChannelID, ChannelConfig:t.ChannelConfig, SigningIdentity:session}

	err = chMgmtClient.SaveChannel(req)
	if err != nil {
		return fmt.Errorf("创建应用通道时发生错误: %s", err)
	}

	time.Sleep(time.Second * 5)

	t.admin, err = t.sdk.NewClient(fabsdk.WithUser(t.OrgAdmin)).ResourceMgmt()
	if err != nil{
		return fmt.Errorf("创建系统资源管理对象发生错误: %s", err)
	}

	err = t.admin.JoinChannel(t.ChannelID)
	if err != nil {
		return fmt.Errorf("将Peers节点加入指定的通道中时发生错误: %s", err)
	}

	fmt.Println("SDK初始化成功")
	t.Initialize = true
	return nil
}


func (t *FabricSetup) InstallAndInstantiateCC() error {
	fmt.Println("开始安装链码......")
	// 创建链码包
	ccPkg, err := gopackager.NewCCPackage(t.ChaincodePath, t.ChaincodeGOPath)
	if err != nil {
		return fmt.Errorf("创建指定的链码包失败: %s", err)
	}

	installCCReq := resmgmtclient.InstallCCRequest{t.ChaincodeID, t.ChaincodePath, Chaincode_Version, ccPkg}

	// 安装链码
	_, err = t.admin.InstallCC(installCCReq)
	if err != nil {
		return fmt.Errorf("安装指定的链码失败: %s", err)
	}

	fmt.Println("安装链码成功")

	fmt.Println("开始实例化链码......")
	// 指定链码策略
	ccPolicy := cauthdsl.SignedByAnyMember([]string{"Org1MSP"})

	// -c '{"Args":["init"]}'
	instantiateCCReq := resmgmtclient.InstantiateCCRequest{Name:t.ChaincodeID, Path:t.ChaincodePath, Version:Chaincode_Version, Args:[][]byte{[]byte("init")}, Policy:ccPolicy}


	err = t.admin.InstantiateCC(t.ChannelID, instantiateCCReq)
	if err != nil {
		return fmt.Errorf("实例化链码失败: %s", err)
	}

	fmt.Println("实例化链码成功")

	t.Client, err = t.sdk.NewClient(fabsdk.WithUser(t.UserName)).Channel(t.ChannelID)
	if err != nil{
		return fmt.Errorf("创建新的通道客户端失败: %s", err)
	}

	fmt.Println("链码安装实例化完成, 且成功创建客户端对象")
	return nil
}


