using UnityEngine;
using UnityEngine.UI;
using System;
using System.Collections.Generic;
using TMPro;

public enum PopupType { Success, Error, Confirm }

public class PopupUI : MonoBehaviour
{
    public TextMeshProUGUI messageText;
    public Button button1;
    public TextMeshProUGUI button1Text;
    public Button button2;
    public TextMeshProUGUI button2Text;
    public Image iconImage; // 타입별 아이콘(선택)

    public void Popup(
        PopupType type,
        string message,
        List<(string, Action)> buttons // (버튼 텍스트, 클릭 이벤트)
    )
    {
        messageText.text = message;
        // 타입별 색상/아이콘 처리 예시
        switch (type)
        {
            case PopupType.Success:
                // iconImage.sprite = ...;
                messageText.color = Color.green;
                break;
            case PopupType.Error:
                // iconImage.sprite = ...;
                messageText.color = Color.red;
                break;
            case PopupType.Confirm:
                // iconImage.sprite = ...;
                messageText.color = Color.white;
                break;
        }

        // 버튼1
        if (buttons.Count > 0)
        {
            button1.gameObject.SetActive(true);
            button1Text.text = buttons[0].Item1;
            button1.onClick.RemoveAllListeners();
            button1.onClick.AddListener(() => {
                buttons[0].Item2?.Invoke();
                Hide();
            });
        }
        else
        {
            button1.gameObject.SetActive(false);
        }

        // 버튼2
        if (buttons.Count > 1)
        {
            button2.gameObject.SetActive(true);
            button2Text.text = buttons[1].Item1;
            button2.onClick.RemoveAllListeners();
            button2.onClick.AddListener(() => {
                buttons[1].Item2?.Invoke();
                Hide();
            });
        }
        else
        {
            button2.gameObject.SetActive(false);
        }

        gameObject.SetActive(true);
    }

    public void Hide()
    {
        gameObject.SetActive(false);
    }
} 