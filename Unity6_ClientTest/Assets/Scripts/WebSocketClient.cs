using UnityEngine;
using NativeWebSocket;
using System;
using System.Text;
using System.Threading.Tasks;
using System.Threading;

public class WebSocketClient : MonoBehaviour
{
    private WebSocket ws;
    public event System.Action<Vector3, float> onMoveApproved;
    public event System.Action<Vector3> onPositionCorrection;
    public event System.Action<bool, string> onRegisterResponse;
    public event System.Action<bool, string> onLoginResponse;

    private int clientId;
    private string username;

    private int maxRetry = 3;
    private int retryCount = 0;
    private int timeoutSeconds = 10;

    private CancellationTokenSource connectCts;
    private bool isConnected = false;

    public void Init(string url, string username)
    {
        this.username = username;
        retryCount = 0;
        isConnected = false;
        connectCts = new CancellationTokenSource();
        _ = ConnectWithRetry(url, connectCts.Token);
    }

    private async Task ConnectWithRetry(string url, CancellationToken token)
    {
        while (retryCount < maxRetry && !isConnected)
        {
            var localCts = CancellationTokenSource.CreateLinkedTokenSource(token);
            var connectTask = Connect(url, localCts.Token);

            if (await Task.WhenAny(connectTask, Task.Delay(timeoutSeconds * 1000, localCts.Token)) == connectTask)
            {
                if (isConnected)
                    break;
            }
            else
            {
                retryCount++;
                if (retryCount > 1 && retryCount <= maxRetry)
                {
                    Debug.LogWarning($"Client {clientId} connect timeout. Retry {retryCount}/{maxRetry}");
                }
                // 재시도 전 약간의 대기 추가
                await Task.Delay(1000, token);
            }
        }

        if (!isConnected)
        {
            Debug.LogError($"Client {clientId} failed to connect after {maxRetry} attempts.");
        }
    }

    private async Task Connect(string url, CancellationToken token)
    {
        if (ws != null)
        {
            try { await ws.Close(); } catch { }
            ws = null;
        }

        ws = new WebSocket(url);

        ws.OnOpen += () =>
        {
            if (!isConnected)
            {
                isConnected = true;
                connectCts?.Cancel(); // 연결 성공 시 재시도 루프 중단
                Debug.Log($"Client {clientId} connected");
                SendLogin();
            }
        };

        ws.OnError += (e) =>
        {
            Debug.LogError($"Client {clientId} error: {e}");
        };

        ws.OnClose += (e) =>
        {
            Debug.Log($"Client {clientId} closed");
        };

        ws.OnMessage += (bytes) =>
        {
            var msg = Encoding.UTF8.GetString(bytes);
            var wsMsg = JsonUtility.FromJson<WSMessage>(msg);

            if (wsMsg.type == "moveApproved")
            {
                var approved = wsMsg.DecodeData<MoveApproved>();
                var target = new Vector3(approved.target.X, approved.target.Y, approved.target.Z);
                onMoveApproved?.Invoke(target, approved.speed);
            }
            else if (wsMsg.type == "positionCorrection")
            {
                var corr = wsMsg.DecodeData<PositionCorrection>();
                var pos = new Vector3(corr.position.X, corr.position.Y, corr.position.Z);
                onPositionCorrection?.Invoke(pos);
            }
            else if (wsMsg.type == "registerResponse")
            {
                var resp = wsMsg.DecodeData<RegisterResponse>();
                onRegisterResponse?.Invoke(resp.success, resp.message);
            }
            else if (wsMsg.type == "loginResponse")
            {
                var resp = wsMsg.DecodeData<LoginResponse>();
                onLoginResponse?.Invoke(resp.success, resp.message);
            }
        };

        try
        {
            await ws.Connect();
        }
        catch (Exception ex)
        {
            Debug.LogError($"Client {clientId} connect exception: {ex.Message}");
        }
    }

    public async void SendLogin()
    {
        if (ws == null || ws.State != WebSocketState.Open)
            return;
        var login = new Login { clientID = clientId, username = username };
        var msg = WSMessage.Create("login", login);
        await ws.SendText(JsonUtility.ToJson(msg));
    }

    public async void SendMoveRequest(Vector3 target)
    {
        if (ws == null || ws.State != WebSocketState.Open)
            return;
        var req = new MoveRequest { target = new Vector { X = target.x, Y = target.y, Z = target.z } };
        var msg = WSMessage.Create("moveRequest", req);
        await ws.SendText(JsonUtility.ToJson(msg));
    }

    public async void SendRegister(string username, string password)
    {
        if (ws == null || ws.State != WebSocketState.Open)
            return;
        var req = new RegisterRequest { username = username, password = password };
        var msg = WSMessage.Create("register", req);
        await ws.SendText(JsonUtility.ToJson(msg));
    }

    public async void SendLogin(string username, string password)
    {
        if (ws == null || ws.State != WebSocketState.Open)
            return;
        var req = new LoginRequest { username = username, password = password };
        var msg = WSMessage.Create("login", req);
        await ws.SendText(JsonUtility.ToJson(msg));
    }

    void Update()
    {
        ws?.DispatchMessageQueue();
    }

    private async void OnDestroy()
    {
        if (ws != null)
        {
            try { await ws.Close(); } catch { }
        }
        connectCts?.Cancel();
    }

    public NativeWebSocket.WebSocketState GetState()
    {
        return ws != null ? ws.State : NativeWebSocket.WebSocketState.Closed;
    }
}
