using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

public class StressTestManager : MonoBehaviour
{
    [Header("테스트 클라이언트 프리팹")]
    public GameObject testClientPrefab; // TestClient 프리팹 (큐브+WebSocketClient)
    [Header("서버 접속 정보")]
    public string serverUrl = "ws://127.0.0.1:9160/ws";
    public int clientCount = 100;
    public string usernamePrefix = "unity_";
    public float sendInterval = 3.0f; // 3초마다 위치 전송

    [Header("클라이언트 생성 딜레이(초)")]
    public float clientCreateDelay = 0.05f; // 추가: 클라이언트 생성 간 딜레이

    [Header("uGUI")]
    public Button startButton;
    public Button stopButton;
    public TMPro.TextMeshProUGUI statusText;

    private List<GameObject> clientObjects = new List<GameObject>();
    private bool running = false;

    void Start()
    {
        if (startButton != null) startButton.onClick.AddListener(StartTest);
        if (stopButton != null) stopButton.onClick.AddListener(StopTest);
        SetButtonState(false);
        UpdateStatus("Waiting");
    }

    public void StartTest()
    {
        if (running) return;
        running = true;
        SetButtonState(true);
        UpdateStatus("Creating clients...");

        // 기존 클라이언트 제거
        foreach (var obj in clientObjects)
            Destroy(obj);
        clientObjects.Clear();

        // clientCount만큼 TestClient 생성 (딜레이 적용)
        StartCoroutine(CreateClientsWithDelay());
        UpdateStatus("Stress test running");
    }

    // 코루틴으로 딜레이 적용 생성
    private System.Collections.IEnumerator CreateClientsWithDelay()
    {
        for (int i = 0; i < clientCount; i++)
        {
            var go = Instantiate(testClientPrefab, UnityEngine.Random.insideUnitSphere * 10f, Quaternion.identity);
            var testClient = go.GetComponent<TestClient>();
            go.name = $"{usernamePrefix}{i}";
            testClient.Init(serverUrl, i, usernamePrefix + i);
            clientObjects.Add(go);
            yield return new WaitForSeconds(clientCreateDelay);
        }
    }

    public void StopTest()
    {
        if (!running) return;
        running = false;
        SetButtonState(false);
        UpdateStatus("Stopping...");

        foreach (var obj in clientObjects)
            Destroy(obj);
        clientObjects.Clear();

        UpdateStatus("Waiting");
    }

    private void UpdateStatus(string msg)
    {
        if (statusText != null)
            statusText.text = msg;
    }

    private void SetButtonState(bool isTesting)
    {
        if (startButton != null) startButton.interactable = !isTesting;
        if (stopButton != null) stopButton.interactable = isTesting;
    }
}