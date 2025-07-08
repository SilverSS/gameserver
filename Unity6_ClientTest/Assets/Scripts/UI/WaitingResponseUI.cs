using UnityEngine;
using TMPro;

public class WaitingResponseUI : MonoBehaviour
{
    public static WaitingResponseUI Instance { get; private set; }

    public CanvasGroup canvasGroup;
    public TextMeshProUGUI messageText;

    private void Awake()
    {
        if (Instance != null && Instance != this)
        {
            Destroy(gameObject);
            return;
        }
        Instance = this;
        DontDestroyOnLoad(gameObject); // 씬 전환에도 유지하고 싶으면 활성화
    }

    void Start()
    {
        Hide();
    }

    public void Show(string message)
    {
        messageText.text = message;
        canvasGroup.alpha = 1f;
        canvasGroup.blocksRaycasts = true;
        canvasGroup.interactable = true;
        gameObject.SetActive(true);
    }

    public void Hide()
    {
        canvasGroup.alpha = 0f;
        canvasGroup.blocksRaycasts = false;
        canvasGroup.interactable = false;
        gameObject.SetActive(false);
    }
} 