#!/usr/bin/env python3
"""Build a simple visual explainer for the agent-based A/B."""

from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd
from matplotlib.patches import FancyArrowPatch, FancyBboxPatch

ROOT = Path(__file__).resolve().parent
AB = ROOT / "hidden_persona_ab.csv"


def box(ax, x, y, w, h, text, color):
    ax.add_patch(FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.02,rounding_size=0.02",
                                linewidth=1.2, edgecolor=color, facecolor=color, alpha=0.12))
    ax.text(x + w / 2, y + h / 2, text, ha="center", va="center", fontsize=10, wrap=True)


def arrow(ax, x1, y1, x2, y2):
    ax.add_patch(FancyArrowPatch((x1, y1), (x2, y2), arrowstyle="-|>", mutation_scale=14,
                                 linewidth=1.4, color="#374151"))


def main() -> None:
    df = pd.read_csv(AB)
    summary = df.groupby("arm").agg(
        match=("match", "mean"),
        preferred=("preferred_share", "mean"),
        purchases=("purchased", "sum"),
    )

    fig = plt.figure(figsize=(14, 10))
    gs = fig.add_gridspec(2, 2, height_ratios=[0.8, 1.2], hspace=0.35, wspace=0.3)

    # Flow diagram
    ax = fig.add_subplot(gs[0, :])
    ax.set_xlim(0, 1)
    ax.set_ylim(0, 1)
    ax.axis("off")
    ax.set_title("Как устроен A/B на агентах: профиль скрыт от рекомендательной системы", fontsize=14, pad=10)
    box(ax, 0.02, 0.45, 0.20, 0.35, "Скрытый профиль\nGo-разработчик\n(не виден сервису)", "#6d28d9")
    box(ax, 0.27, 0.45, 0.20, 0.35, "Агент действует\nпо профилю:\nклик / покупка", "#b45309")
    box(ax, 0.52, 0.45, 0.20, 0.35, "Сервис видит только\nclient_id, покупки\nи события", "#1d4ed8")
    box(ax, 0.77, 0.45, 0.20, 0.35, "Feedback + bandit\nменяют следующую\nвыдачу", "#047857")
    arrow(ax, 0.22, 0.62, 0.27, 0.62)
    arrow(ax, 0.47, 0.62, 0.52, 0.62)
    arrow(ax, 0.72, 0.62, 0.77, 0.62)
    arrow(ax, 0.87, 0.45, 0.62, 0.30)
    ax.text(0.74, 0.26, "следующая сессия", fontsize=9, color="#6b7280")
    ax.text(0.02, 0.08, "Профиль не передаётся в API. Система учится только по поведению.", fontsize=9, color="#6b7280")

    # Offline backtest chart
    ax = fig.add_subplot(gs[1, 0])
    labels = ["B2B A", "B2B B", "B2C A", "B2C B"]
    baseline = [3.6, 5.1, 19.4, 19.2]
    ours = [9.87, 12.32, 19.44, 19.23]
    x = range(len(labels))
    ax.bar([i - 0.18 for i in x], baseline, width=0.36, label="baseline", color="#c8102e")
    ax.bar([i + 0.18 for i in x], ours, width=0.36, label="наша модель", color="#1d4ed8")
    for i, v in enumerate(baseline):
        ax.text(i - 0.18, v + 0.3, f"{v:.1f}", ha="center", fontsize=9)
    for i, v in enumerate(ours):
        ax.text(i + 0.18, v + 0.3, f"{v:.1f}", ha="center", fontsize=9)
    ax.set_xticks(list(x), labels)
    ax.set_ylabel("Precision@3, %")
    ax.set_title("Офлайн-бэктест на датасете v3", fontsize=12)
    ax.legend()
    ax.grid(axis="y", alpha=0.25)

    # Agent A/B chart
    ax = fig.add_subplot(gs[1, 1])
    metrics = ["match", "preferred", "purchases"]
    titles = ["Совпадение\nс профилем", "Доля\nпредпочтённых", "Покупки\nза 100 сессий"]
    pop = [summary.loc["popular", "match"], summary.loc["popular", "preferred"], summary.loc["popular", "purchases"] / 100]
    hyb = [summary.loc["hybrid", "match"], summary.loc["hybrid", "preferred"], summary.loc["hybrid", "purchases"] / 100]
    x = range(len(metrics))
    ax.bar([i - 0.18 for i in x], pop, width=0.36, label="popular", color="#c8102e")
    ax.bar([i + 0.18 for i in x], hyb, width=0.36, label="hybrid + feedback", color="#1d4ed8")
    for i in x:
        ax.text(i - 0.18, pop[i] + 0.02, f"{pop[i]*100:.0f}%", ha="center", fontsize=9)
        ax.text(i + 0.18, hyb[i] + 0.02, f"{hyb[i]*100:.0f}%", ha="center", fontsize=9)
    ax.set_xticks(list(x), titles)
    ax.set_ylim(0, 1.08)
    ax.set_ylabel("доля / match")
    ax.set_title("A/B на одном агенте: 100 сессий", fontsize=12)
    ax.legend()
    ax.grid(axis="y", alpha=0.25)

    fig.suptitle("Проверка рекомендаций на агентах: без реальных людей", fontsize=16, y=0.98)
    out = ROOT / "ab_explainer.png"
    fig.savefig(out, dpi=170, bbox_inches="tight")
    print("saved", out)
    print(summary.to_string())


if __name__ == "__main__":
    main()
