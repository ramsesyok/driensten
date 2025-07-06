import socket
import threading
import time


def udp_listener(host: str, port: int):
    """
    host: 待ち受けアドレス
    port: 待ち受けポート番号
    受信したUDPペイロードをUTF-8文字列として出力する
    """
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind((host, port))
    print(f"[Listener] Listening on {host}:{port}")
    while True:
        data, addr = sock.recvfrom(2048)  # バッファサイズは適宜調整
        print("receved\n")
        try:
            message = data.decode("utf-8")
        except UnicodeDecodeError:
            message = data.decode("utf-8", errors="replace")
        print(f"[Listener] Received from {addr}:")
        print(message)
        print("-" * 40)


def udp_sender(host: str, port: int, message: str, interval: float):
    """
    host: 送信先アドレス
    port: 送信先ポート番号
    message: 送信文字列（\n を含む場合もOK）
    interval: 送信間隔（秒）
    """
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    print(f"[Sender] Sending to {host}:{port} every {interval} second(s)")
    while True:
        sent = sock.sendto(message.encode("utf-8"), (host, port))
        # sendto が返すのはバイト単位の送信長
        # print(f"[Sender] Sent {sent} bytes")
        time.sleep(interval)


def main():
    # 受信スレッドをデーモンとして起動
    listener_thread = threading.Thread(
        target=udp_listener, args=("127.0.0.1", 5653), daemon=True
    )
    listener_thread.start()

    # 送信処理はメインスレッドで実行
    message = "topicA\nGood Bye\nnice;to:meet,you!"
    udp_sender("127.0.0.1", 6565, message, interval=1.0)


if __name__ == "__main__":
    main()
