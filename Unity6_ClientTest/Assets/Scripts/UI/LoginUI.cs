using UnityEngine;
using TMPro;
using UnityEngine.UI;
using System.Collections;
using UnityEngine.Networking;

public class LoginUI : MonoBehaviour
{
    public TMP_InputField usernameInput;
    public TMP_InputField passwordInput;
    public Button loginButton;
    public Button registerButton;
    public GameObject loginPanel;

    public WebSocketClient wsClient;
    public PopupUI popupUI;
    public WaitingResponseUI waitingUI;

    public string serverHttpUrl = "http://localhost:9160"; // REST API 주소
    public string wsUrl = "ws://localhost:9160/ws"; // WebSocket 주소

    //private bool isConnected = false;

    void Start()
    {
        loginButton.onClick.RemoveAllListeners();
        registerButton.onClick.RemoveAllListeners();
        loginButton.onClick.AddListener(OnLoginClicked);
        registerButton.onClick.AddListener(OnRegisterClicked);
        popupUI.Hide();
        waitingUI.Hide();
    }

    void OnEnable()
    {
        wsClient.onLoginResponse += HandleLoginResponse;
        wsClient.onRegisterResponse += HandleRegisterResponse;
        wsClient.onMoveApproved += OnMoveApprovedDummy;
        wsClient.onPositionCorrection += OnPositionCorrectionDummy;
    }

    void OnDisable()
    {
        wsClient.onLoginResponse -= HandleLoginResponse;
        wsClient.onRegisterResponse -= HandleRegisterResponse;
        wsClient.onMoveApproved -= OnMoveApprovedDummy;
        wsClient.onPositionCorrection -= OnPositionCorrectionDummy;
    }

    private void OnLoginClicked()
    {
        Debug.Log("[LoginUI] 로그인 버튼 클릭됨");
        loginButton.interactable = false;
        StartCoroutine(LoginRequest());
    }

    private void OnRegisterClicked()
    {
        StartCoroutine(RegisterRequest());
    }

    private IEnumerator LoginRequest()
    {
        string username = usernameInput.text.Trim();
        string password = passwordInput.text;
        Debug.Log($"[LoginUI] 로그인 요청: {username}, {password}");
        if (string.IsNullOrEmpty(username) || string.IsNullOrEmpty(password))
        {
            popupUI.Popup(PopupType.Error, "아이디와 비밀번호를 입력하세요.", new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
            loginButton.interactable = true;
            yield break;
        }
        waitingUI.Show("로그인 중...");

        WWWForm form = new WWWForm();
        form.AddField("username", username);
        form.AddField("password", password);
        using (UnityWebRequest www = UnityWebRequest.Post(serverHttpUrl + "/login", form))
        {
            yield return www.SendWebRequest();
            waitingUI.Hide();
            loginButton.interactable = true;
            if (www.result != UnityWebRequest.Result.Success)
            {
                popupUI.Popup(PopupType.Error, "서버 오류: " + www.error, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
            }
            else
            {
                var response = www.downloadHandler.text;
                var loginResp = JsonUtility.FromJson<LoginResponse>(response);
                if (loginResp.success)
                {
                    wsClient.Init(wsUrl + "?token=" + loginResp.token, username);
                    popupUI.Popup(PopupType.Success, "로그인 성공!", new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
                }
                else
                {
                    Debug.Log("서버 응답: " + response);
                    Debug.Log("파싱된 메시지: " + loginResp.message);
                    string msg = string.IsNullOrEmpty(loginResp.message) ? "알 수 없는 오류가 발생했습니다." : loginResp.message;
                    popupUI.Popup(PopupType.Error, msg, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
                }
            }
        }
    }

    private IEnumerator RegisterRequest()
    {
        string username = usernameInput.text.Trim();
        string password = passwordInput.text;
        if (string.IsNullOrEmpty(username) || string.IsNullOrEmpty(password))
        {
            popupUI.Popup(PopupType.Error, "아이디와 비밀번호를 입력하세요.", new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
            yield break;
        }
        waitingUI.Show("회원가입 중...");

        WWWForm form = new WWWForm();
        form.AddField("username", username);
        form.AddField("password", password);
        using (UnityWebRequest www = UnityWebRequest.Post(serverHttpUrl + "/register", form))
        {
            yield return www.SendWebRequest();
            waitingUI.Hide();
            if (www.result != UnityWebRequest.Result.Success)
            {
                popupUI.Popup(PopupType.Error, "서버 오류: " + www.error, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
            }
            else
            {
                var response = www.downloadHandler.text;
                var regResp = JsonUtility.FromJson<RegisterResponse>(response);
                popupUI.Popup(regResp.success ? PopupType.Success : PopupType.Error, regResp.message, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
            }
        }
    }

    private void HandleLoginResponse(bool success, string msg)
    {
        waitingUI.Hide();
        popupUI.Popup(success ? PopupType.Success : PopupType.Error, msg, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
    }

    private void HandleRegisterResponse(bool success, string msg)
    {
        waitingUI.Hide();
        popupUI.Popup(success ? PopupType.Success : PopupType.Error, msg, new System.Collections.Generic.List<(string, System.Action)>{ ("확인", null) });
    }

    private void OnMoveApprovedDummy(Vector3 t, float s) { }
    private void OnPositionCorrectionDummy(Vector3 v) { }
} 