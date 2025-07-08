using System;
using System.Text;

[System.Serializable]
public class WSMessage
{
    public string type;
    public string data; // base64 인코딩된 문자열

    // 객체를 base64 data로 감싼 WSMessage 생성
    public static WSMessage Create<T>(string type, T obj)
    {
        string json = UnityEngine.JsonUtility.ToJson(obj);
        string base64 = Convert.ToBase64String(Encoding.UTF8.GetBytes(json));
        return new WSMessage { type = type, data = base64 };
    }

    // data(base64)를 원하는 타입으로 디코딩
    public T DecodeData<T>()
    {
        var json = Encoding.UTF8.GetString(Convert.FromBase64String(data));
        return UnityEngine.JsonUtility.FromJson<T>(json);
    }
}

[System.Serializable]
public class Login
{
    public int clientID;
    public string username;
}

[System.Serializable]
public class Vector
{
    public float X;
    public float Y;
    public float Z;
}

[System.Serializable]
public class PlayerState
{
    public int health;
    public Vector Position;
    public Vector Target; // 목표 위치
    public int moveState; // 0: Idle, 1: Moving
}

[System.Serializable]
public class MoveRequest
{
    public Vector target;
}

[System.Serializable]
public class MoveApproved
{
    public Vector target;
    public float speed;
}

[System.Serializable]
public class PositionCorrection
{
    public Vector position;
}

[System.Serializable]
public class RegisterRequest
{
    public string username;
    public string password;
}

[System.Serializable]
public class RegisterResponse
{
    public bool success;
    public string message;
}

[System.Serializable]
public class LoginRequest
{
    public string username;
    public string password;
}

[System.Serializable]
public class LoginResponse
{
    public bool success;
    public string message;
    public string token; // JWT 토큰
}