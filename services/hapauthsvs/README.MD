## 密码认证服务
### 基本信息

| 说明   | 用于账号密码登录认证的服务 |
| ------ | -------------------------- |
| 服务名 | ap                         |

### 配置文件



#### 配置项

```ini
[ApAuthService]
```

#### 配置参数：

| 名称                | 必填 | 说明   | 示例（缺省） |
| ------------------- | ---- | ------ | ------------ |
| name                | YES  | 服务名 | ap           |
| session_service     | YES | session token服务名 | session |
| session_create_slot | YES | 创建session token的slot名 | createToken |
| session_verify_slot | YES | 验证session token的slot名 | verifyToken |
| session_revoke_slot | YES | 撤销session token的slot名 | revokeToken |
| pwd_encoding        | NO | 接收密码参数的编码类型（如果不设置，则表示密码是明文） | base64 |
| pwd_secret          | NO | 解码密码参数所用密钥 |              |
| super               | NO | 超级用户密码（经过特殊编码，需要用专门工具生成） | 89c766f8cf1624a178f4c8cf599d978b |
| lock_after_fails    | NO | 几次登录以后锁定账号 | 5 |
| pwd_min_len         | NO | 密码最小长度 | 6 |
| pwd_max_len         | NO | 密码最大长度 | 8 |
| pwd_upper_and_lower_letter         | NO | 密码是否包含大小写字母 | true |
| pwd_number_and_letter         | NO | 密码是否包含数字和字母 | true |
| pwd_has_symbol         | NO | 密码是否包含特殊符号 | true |
| pwd_symbols         | NO | 密码允许的特殊符号（允许经过base64编码） | {{base64:JF4mKigpPXx7fSc6IVtdXy0rfi48Pg==}} |
| default_pwd         | YES | 重置密码时的缺省密码（不需要经过编码） | Qaz@2020 |


#### 配置文件样例

```ini
[ApAuthService]
name = ap
session_service = session
session_create_slot = createToken
session_verify_slot = verifyToken
session_revoke_slot = revokeToken
pwd_encoding = base64
pwd_secret =
super = 89c766f8cf1624a178f4c8cf599d978b
lock_after_fails = 5
pwd_min_len = 6
pwd_max_len = 8
pwd_upper_and_lower_letter = true
pwd_number_and_letter = true
pwd_has_symbol = true
pwd_symbols = {{base64:JF4mKigpPXx7fSc6IVtdXy0rfi48Pg==}}
default_pwd = Qaz@2020
```


### 接口定义

#### 定义文件名称
	ap.slot


#### 槽方法
##### login
参数说明：

| 名称 | 类型   | 必填 | 说明   | 备注 |
| ---- | ------ | ---- | ------ | ---- |
| user | string | YES  | 用户名 | ap   |



#### 接口文件样例

```json
[
  {
    "name": "login",
    "lang": "go",
    "impl": "Login",
    "params": [
      {
        "desc": "用户名",
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "desc": "密码",
        "name": "password",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "checkLogin",
    "lang": "go",
    "impl": "CheckLogin",
    "params": [
      {
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "name": "token",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "logout",
    "lang": "go",
    "impl": "Logout",
    "params": [
      {
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "name": "token",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "changeSuperPwd",
    "lang": "go",
    "impl": "ChangeSuperPwd",
    "params": [
      {
        "name": "old_password",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "name": "new_password",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "addUser",
    "lang": "go",
    "impl": "AddUser",
    "params": [
      {
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "name": "password",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "delUser",
    "lang": "go",
    "impl": "DelUser",
    "params": [
      {
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      }
    ]
  },
  {
    "name": "updateUser",
    "lang": "go",
    "impl": "UpdateUser",
    "params": [
      {
        "name": "user",
        "type": "string",
        "required": true,
        "validator": ""
      },
      {
        "name": "password",
        "type": "string",
        "required": false,
        "validator": ""
      },
      {
        "name": "locked",
        "type": "bool",
        "required": false,
        "validator": ""
      }
    ]
  },
  {
    "name": "users",
    "lang": "go",
    "impl": "GetUsers",
    "params": [
      {
        "name": "paging",
        "type": "numbound",
        "required": true,
        "validator": ""
      }
    ]
  }
]
```
