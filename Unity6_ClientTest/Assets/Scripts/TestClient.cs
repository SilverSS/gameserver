using UnityEngine;
using System.Collections;
using System.Collections.Generic;

public class TestClient : MonoBehaviour
{
    public WebSocketClient wsClient;
    private Vector3 targetPosition;
    private float moveSpeed = 1f;
    private bool moving = false;

    public void Init(string url, int clientId, string username)
    {
        wsClient.Init(url, clientId, username, OnMoveApproved, OnPositionCorrection);
        // 최초 이동 요청
        StartCoroutine(MoveRequestRoutine());
    }

    // 주기적으로 임의의 목표 위치로 이동 요청
    private IEnumerator MoveRequestRoutine()
    {
        while (true)
        {
            yield return new WaitForSeconds(3f);
            var randomTarget = transform.position + UnityEngine.Random.insideUnitSphere * 10f;
            wsClient.SendMoveRequest(randomTarget);
        }
    }

    // 서버로부터 이동 승인 메시지 수신 시
    private void OnMoveApproved(Vector3 target, float speed)
    {
        targetPosition = target;
        moveSpeed = speed;
        moving = true;
        //Debug.Log($"Move approved to {target} at speed {speed}");
    }

    // 서버로부터 위치 보정 메시지 수신 시
    private void OnPositionCorrection(Vector3 serverPos)
    {
        //Debug.Log($"Position correction received: {serverPos}");
        // 서버 위치와 차이가 크면 보정
        if ((serverPos - transform.position).sqrMagnitude > 0.05f)
        {
            transform.position = serverPos;
            //Debug.Log($"Position corrected to {serverPos}");
        }
    }

    void Update()
    {
        if (moving)
        {
            float step = moveSpeed * Time.deltaTime;
            transform.position = Vector3.MoveTowards(transform.position, targetPosition, step);
            if (Vector3.Distance(transform.position, targetPosition) < 0.01f)
            {
                moving = false;
            }
        }
    }
}

public class TestClientManager : MonoBehaviour
{
    public GameObject testClientPrefab;
    public string serverUrl = "ws://localhost:8080";
    public string usernamePrefix = "User_";
    public int clientCount = 10;

    private List<GameObject> clientObjects = new List<GameObject>();

    void Start()
    {
        for (int i = 0; i < clientCount; i++)
        {
            var go = Instantiate(testClientPrefab, UnityEngine.Random.insideUnitSphere * 10f, Quaternion.identity);
            var testClient = go.GetComponent<TestClient>();
            testClient.Init(serverUrl, i, usernamePrefix + i);
            clientObjects.Add(go);
        }
    }
}