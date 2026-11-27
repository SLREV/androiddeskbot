package com.grokbot.desk;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import botlib.Botlib;

/** Shell tipis: start server Go, tampilkan night desk di WebView. */
public class MainActivity extends Activity {
    private WebView webView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        webView = new WebView(this);
        setContentView(webView);
        webView.getSettings().setJavaScriptEnabled(true);
        webView.getSettings().setDomStorageEnabled(true);
        webView.setWebViewClient(new WebViewClient());

        String url = Botlib.start(getFilesDir().getAbsolutePath());
        if (url == null || url.isEmpty()) {
            webView.loadData(
                "<body style='background:#0b0d0c;color:#eaf3ee;font-family:monospace;text-align:center;padding-top:40vh'>" +
                "Gagal menjalankan Grok Bot.</body>", "text/html", "utf-8");
            return;
        }
        webView.loadUrl(url);
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        Botlib.stop();
        super.onDestroy();
    }
}
