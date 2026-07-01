import "./styles.css";

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <main class="shell">
    <section class="hero">
      <p class="eyebrow">混沌人生默认开启</p>
      <h1>250 万清空挑战</h1>
      <p class="lead">目标很简单：尽快花光 2,500,000 元。系统会用退款、中奖、疾病、误操作、意外事件不断打乱你。</p>
      <button class="primary-button" type="button">开始挑战</button>
    </section>
  </main>
`;
