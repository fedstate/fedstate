package mgo

// mongo script
const (
	// %s里不能使用单引号(')
	MongoShellEvalWithAuth = `mongo -u root -p '%s' --eval '%s'`
	MongoShellEvalNoAuth   = `mongo --eval '%s'`

	// success:
	// "ok" : 1
	// fail:
	// {"info":"try querying local.system.replset to see current configuration","ok":0,"errmsg":"already initialized","code":23,"codeName":"AlreadyInitialized"}
	// errmsg\" : \"Our config version of 1 is no larger than the version on 10.29.5.107:30247, which is 1\",\
	RSIntitate = `rs.initiate({_id: "%s", members: %s});`
	// 当多个节点共同初始化时，会生成不一样的replicaSetId
	RSReconfig = `rs.reconfig({_id: "%s", members: %s, force: true });`

	DBServerStatusReplMe = `db.serverStatus().repl.me;`
	// success:
	// "ok" : 1
	// fail:
	// "ok" : 0
	RSStatus             = `rs.status();`
	RSAlreadyInitialized = `AlreadyInitialized`
	RSConfigIncompatible = `NewReplicaSetConfigurationIncompatible`

	OK = `"ok" : 1`

	// success:
	// Successfully added user
	// fail:
	// Error: couldn't add user: there are no users authenticated
	CreateUser = `
db.getSiblingDB("admin").createUser({
    user: "%s",
    pwd: "%s",
    roles: [{role: "root", db: "admin"}]
});
`
	CreateUserSuccess = `Successfully added user`
	// 用户已经创建成功
	CreateUserUnauthorized = `no users authenticated`

	ReconfigUnauthorized = `not authorized on admin to execute command`
	// 在非master节点上执行了创建root用户
	CreateUserNotMaster = `not master`
)
