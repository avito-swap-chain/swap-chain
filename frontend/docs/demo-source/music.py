"""Генерирует фоновую подложку для ролика: public/music.wav.

    python3 music.py [секунды]

Своя музыка, а не сток: лицензия чистая, длительность подгоняется под ролик,
и трек намеренно тихий и однообразный — он держит темп, но не спорит с экраном.
Тембр — затухающие гармоники, как у электропиано; аккорды меняются раз в такт.
"""

import math
import struct
import sys
import wave

import numpy as np

RATE = 44100
BAR = 3.4  # секунд на аккорд: медленнее речи, чтобы не подгонять зрителя
GAIN = 0.16  # подложка, а не саундтрек

# Am — F — C — G: нейтральная последовательность без драматизма.
CHORDS = [
    (57, 60, 64),  # Am
    (53, 57, 60),  # F
    (48, 52, 55),  # C
    (55, 59, 62),  # G
]


def hz(midi: int) -> float:
    return 440.0 * 2 ** ((midi - 69) / 12)


def voice(freq: float, length: int, offset: float) -> np.ndarray:
    """Одна нота: основной тон плюс две тихие гармоники с общим затуханием."""
    t = np.arange(length) / RATE
    envelope = np.exp(-t * 1.15) * (1 - np.exp(-t * 60))
    wave_form = (
        np.sin(2 * np.pi * freq * t + offset)
        + 0.35 * np.sin(4 * np.pi * freq * t)
        + 0.12 * np.sin(6 * np.pi * freq * t)
    )
    return wave_form * envelope


def build(seconds: float) -> np.ndarray:
    total = int(seconds * RATE)
    left = np.zeros(total + RATE * 4)
    right = np.zeros_like(left)

    bar_len = int(BAR * RATE)
    for index in range(math.ceil(seconds / BAR) + 1):
        chord = CHORDS[index % len(CHORDS)]
        start = index * bar_len

        for note_index, midi in enumerate(chord):
            # Ноты аккорда вступают с небольшим сдвигом — звучит живее одновременного удара.
            delay = int(note_index * 0.055 * RATE)
            at = start + delay
            tone = voice(hz(midi), bar_len * 2, offset=note_index * 0.7)
            fits = min(len(tone), len(left) - at)
            # Стерео: голоса чуть разведены по панораме.
            pan = 0.5 + (note_index - 1) * 0.16
            left[at : at + fits] += tone[:fits] * (1 - pan)
            right[at : at + fits] += tone[:fits] * pan

    stereo = np.stack([left[:total], right[:total]])

    # Мягкие края: ролик начинается и заканчивается без щелчка.
    fade_in, fade_out = int(1.5 * RATE), int(2.5 * RATE)
    stereo[:, :fade_in] *= np.linspace(0, 1, fade_in)
    stereo[:, -fade_out:] *= np.linspace(1, 0, fade_out)

    peak = np.max(np.abs(stereo))
    return stereo / peak * GAIN if peak else stereo


def main() -> None:
    seconds = float(sys.argv[1]) if len(sys.argv) > 1 else 44.0
    stereo = build(seconds)
    frames = (stereo.T.reshape(-1) * 32767).astype(np.int16)

    with wave.open("public/music.wav", "w") as out:
        out.setnchannels(2)
        out.setsampwidth(2)
        out.setframerate(RATE)
        out.writeframes(struct.pack(f"<{len(frames)}h", *frames))

    print(f"public/music.wav — {seconds:.0f} c")


if __name__ == "__main__":
    main()
