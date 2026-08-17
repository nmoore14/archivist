import fs from "node:fs/promises";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const OUT = "/Users/nickmoore/Desktop/school/summer_2026/msai_699/archivist/Archivist_Project_Presentation.pptx";
const BUILD = "/Users/nickmoore/Desktop/school/summer_2026/msai_699/archivist/.presentation-build";
const ROOT = "/Users/nickmoore/Desktop/school/summer_2026/msai_699/archivist";
const W = 1280, H = 720;
const C = { ink: "#101214", muted: "#59616B", light: "#EEF1F3", rule: "#BCC3C9", blue: "#2F80ED", cyan: "#75D4F5", navy: "#173B57", white: "#FFFFFF", green: "#2E8B70", amber: "#D97706" };
const FONT = "Arial";

const p = Presentation.create({ slideSize: { width: W, height: H } });

function rect(slide, left, top, width, height, fill=C.light, line="none", radius=0) {
  return slide.shapes.add({ geometry: radius ? "roundRect" : "rect", position:{left,top,width,height}, fill, line:{style:"solid",fill:line,width:line==="none"?0:1}, ...(radius?{borderRadius:radius}:{}) });
}
function text(slide, value, left, top, width, height, size=24, color=C.ink, bold=false, align="left") {
  const s = slide.shapes.add({ geometry:"textbox", position:{left,top,width,height}, fill:"none", line:{style:"solid",fill:"none",width:0} });
  s.text = value;
  s.text.style = { fontFamily:FONT, fontSize:size, color, bold, alignment:align, verticalAlignment:"middle" };
  return s;
}
function title(slide, value, kicker) {
  text(slide, kicker.toUpperCase(), 56, 34, 430, 24, 13, C.muted, true);
  text(slide, value, 56, 68, 1168, 64, 38, C.ink, true);
  rect(slide,56,146,1168,2,C.ink);
}
function footer(slide, n) {
  text(slide, "ARCHIVIST  •  LOCAL-FIRST AI FOR SCHOOLS", 56, 678, 500, 18, 10, C.muted, true);
  text(slide, String(n).padStart(2,"0"), 1160, 678, 64, 18, 10, C.muted, true, "right");
}
function note(slide, body, sources=[]) {
  const src = sources.length ? `\n\n[Sources]\n${sources.map(x=>`- ${x}`).join("\n")}\n[/Sources]` : "\n\n[Sources]\n- Project repository artifacts and evaluation outputs supplied in the workspace.\n[/Sources]";
  slide.speakerNotes.textFrame.setText(body + src);
  slide.speakerNotes.setVisible(true);
}
async function imageBytes(path){ const b=await fs.readFile(path); return b.buffer.slice(b.byteOffset,b.byteOffset+b.byteLength); }

// 1 — cover
{
  const s=p.slides.add(); s.background.fill=C.white;
  text(s,"MSAI 699 CAPSTONE",56,38,360,24,14,C.muted,true);
  text(s,"Archivist",56,188,700,92,76,C.ink,true);
  text(s,"A local-first AI workspace that keeps course answers grounded in trusted material",56,298,760,104,30,C.navy,false);
  rect(s,930,0,350,720,C.navy);
  rect(s,970,150,245,245,C.cyan,0,24);
  text(s,"PRIVATE",994,185,197,44,22,C.navy,true,"center");
  text(s,"+",994,243,197,44,32,C.navy,true,"center");
  text(s,"GROUNDED",994,305,197,44,22,C.navy,true,"center");
  text(s,"Nick Moore  •  Summer 2026",56,620,500,26,18,C.muted,false);
  note(s,"Stakeholders need educational AI that is useful without sending sensitive course content to an external service. Archivist explores a practical middle path: local deployment, guided administration, and answers tied directly to instructor-approved sources. Today I’ll show the problem, the system, how I evaluated it, and what the results suggest.");
}

// 2 — problem
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"Schools face an AI access–control tradeoff","The problem");
  text(s,"Cloud assistants are easy to try, but they can create concerns around privacy, source control, cost, and reliability.",56,178,640,92,27,C.ink,true);
  rect(s,56,316,540,238,C.light);
  text(s,"What stakeholders need",84,340,450,36,23,C.navy,true);
  text(s,"• Course content stays under local control\n• Answers trace back to approved material\n• Setup works without an AI infrastructure team",84,395,455,130,20,C.ink,false);
  text(s,"The research question",760,190,360,28,18,C.muted,true);
  text(s,"Can guided workflows reduce the technical barriers to deploying local AI assistants for education?",760,240,390,220,36,C.blue,true);
  footer(s,2);
  note(s,"The project began with a stakeholder tension. Schools want the benefits of conversational AI, but administrators also need control over data, sources, and deployment. Local models help with control, yet local AI is usually harder to configure. That leads to the project’s research question: can a guided workflow make local AI practical for an educational setting?");
}

// 3 — solution workflow
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"Archivist turns trusted course files into cited student answers","The solution");
  const xs=[64,360,656,952]; const labels=["1  ADMIN","2  INDEX","3  ASK","4  ANSWER"]; const bodies=["Create a course workspace and upload approved documents.","Extract, chunk, and embed content using local models.","Assigned students ask questions in a simple web interface.","Generate a grounded response with references to source material."];
  for(let i=0;i<4;i++){
    rect(s,xs[i],214,236,250,i===3?"#DDF4FC":C.light);
    text(s,labels[i],xs[i]+22,234,192,28,16,i===3?C.blue:C.muted,true);
    text(s,bodies[i],xs[i]+22,294,192,125,22,C.ink,i===3);
    if(i<3) text(s,"→",xs[i]+243,302,46,50,34,C.blue,true,"center");
  }
  text(s,"One guided path connects administration, retrieval, and learning—without moving course data to a hosted AI service.",120,520,1040,60,25,C.navy,true,"center");
  footer(s,3);
  note(s,"The workflow is deliberately simple. An administrator creates a workspace, uploads trusted material, and assigns students. Archivist extracts the text, breaks it into chunks, and creates embeddings locally. When a student asks a question, the system retrieves the most relevant chunks, sends that context to the local language model, and returns an answer with source references. The value is not just the model—it is the guided end-to-end workflow.");
}

// 4 — methodology
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"The evaluation separated retrieval value from tuning","Methodology");
  text(s,"BASELINE STUDY",56,190,280,30,18,C.blue,true);
  text(s,"15 versioned questions",56,235,360,45,31,C.ink,true);
  text(s,"Compared no-RAG and RAG answers with manual correctness scoring, groundedness, retrieval hits, hallucination handling, and latency.",56,296,470,156,20,C.ink,false);
  rect(s,610,182,2,382,C.rule);
  text(s,"OPTIMIZATION STUDY",672,190,300,30,18,C.blue,true);
  text(s,"6 RAG configurations",672,235,420,45,31,C.ink,true);
  text(s,"Varied chunk size, overlap, and top-k while holding the same question set and using a project-specific quality–speed selection score.",672,296,470,156,20,C.ink,false);
  rect(s,56,500,1088,58,"#FFF3D8");
  text(s,"Scope: a small initial benchmark—not a statistical or production-scale validation.",76,511,1048,36,18,C.amber,true,"center");
  footer(s,4);
  note(s,"I used a two-stage methodology. First, the baseline study compared the model without retrieval against the same model with retrieval on 15 versioned questions. I manually scored correctness and tracked groundedness, retrieval success, hallucination handling, and response time. Second, I evaluated six retrieval configurations by changing chunk size, overlap, and the number of retrieved chunks. The same test set made the comparisons reproducible. Because the sample is small, I treat these results as directional rather than statistically significant.");
}

// 5 — baseline result
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"Grounding raised answer correctness by 30 percentage points","Baseline results");
  text(s,"NO RETRIEVAL",76,192,360,24,16,C.muted,true,"center");
  text(s,"63.3%",76,230,360,92,64,C.ink,true,"center");
  text(s,"correctness",76,326,360,30,20,C.muted,false,"center");
  text(s,"+30.0 pts",470,253,340,58,36,C.blue,true,"center");
  text(s,"WITH RETRIEVAL",844,192,360,24,16,C.muted,true,"center");
  text(s,"93.3%",844,230,360,92,64,C.blue,true,"center");
  text(s,"correctness",844,326,360,30,20,C.muted,false,"center");
  rect(s,124,420,1032,2,C.rule);
  const stats=[["100%","Retrieval Hit Rate@3"],["0.93","Average groundedness"],["100%","Unanswerable-question handling"]];
  stats.forEach((v,i)=>{const x=92+i*386;text(s,v[0],x,458,320,56,34,i===1?C.blue:C.navy,true,"center");text(s,v[1],x,520,320,42,17,C.muted,false,"center");});
  footer(s,5);
  note(s,"The baseline result is the clearest evidence in the project. Without retrieval, answers earned 63.3 percent of the maximum correctness score. With retrieval, correctness rose to 93.3 percent—a 30-point improvement. Retrieval found relevant material for every question, average groundedness was 0.93, and the system correctly handled the questions whose answers were not in the source. The important stakeholder takeaway is that trusted context did more than add citations; it materially improved the answer behavior.");
}

// 6 — tuning
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"Tuning improved quality and cut mean latency 43%","Optimization results");
  const img=await imageBytes(`${ROOT}/results/charts/correctness_latency_tradeoff.png`);
  s.images.add({blob:img,contentType:"image/png",alt:"Correctness and response-time tradeoff across six RAG configurations",fit:"contain",position:{left:56,top:172,width:700,height:430}});
  text(s,"SELECTED",810,186,300,24,16,C.blue,true);
  text(s,"top-k = 1",810,222,350,46,32,C.ink,true);
  rect(s,810,296,350,2,C.rule);
  text(s,"96.7%",810,320,160,54,34,C.blue,true);
  text(s,"correctness",810,374,160,28,16,C.muted,false);
  text(s,"1.07 s",1000,320,160,54,34,C.navy,true);
  text(s,"mean response",1000,374,160,28,16,C.muted,false);
  text(s,"Versus baseline: +3.3 correctness points and −0.81 seconds.",810,438,350,92,22,C.ink,true);
  text(s,"Smaller chunks reached 100% correctness, but the selection score favored top-k 1 for its stronger quality–speed balance.",810,538,350,82,16,C.muted,false);
  footer(s,6);
  note(s,"The optimization study showed that more retrieved context was not automatically better. The selected configuration kept the original 1,000-character chunks and 200-character overlap, but reduced top-k from three chunks to one. It reached 96.7 percent correctness with a mean response time of 1.07 seconds. Relative to the baseline configuration, that is a 3.3-point correctness improvement and a 43 percent reduction in mean latency. A smaller-chunk setup did reach 100 percent correctness, but the project’s combined quality-and-speed score favored top-k one.");
}

// 7 — meaning and limitations
{
  const s=p.slides.add(); s.background.fill=C.white; title(s,"The MVP validates the technical path—not usability","Interpretation");
  text(s,"WHAT THE RESULTS SUPPORT",56,184,500,26,17,C.blue,true);
  text(s,"Retrieval makes local answers substantially more reliable on the project test set.\n\nCareful tuning can reduce context and latency without sacrificing quality.\n\nA guided local workflow is feasible as a deployable educational MVP.",56,228,510,330,23,C.ink,false);
  rect(s,620,176,2,404,C.rule);
  text(s,"WHAT THEY DO NOT YET PROVE",684,184,500,26,17,C.amber,true);
  text(s,"The 15-question benchmark is too small for broad generalization.\n\nAI-assisted quality scores still require final human verification.\n\nThe central research question needs user testing: completion rate, time on task, errors, recovery, and confidence.",684,228,510,330,23,C.ink,false);
  footer(s,7);
  note(s,"These results support three conclusions: retrieval is valuable for this use case, configuration choices matter, and the local architecture can deliver responsive grounded answers. But they do not yet prove that nontechnical administrators can deploy the system successfully. The evaluation is small and project-specific, and the optimization quality scores are an AI-assisted draft pending human verification. The next study should focus directly on the guided workflow using completion rate, time on task, errors, recovery behavior, and confidence.");
}

// 8 — close
{
  const s=p.slides.add(); s.background.fill=C.navy;
  text(s,"CONCLUSION",56,42,320,24,14,C.cyan,true);
  text(s,"Archivist makes local educational AI more useful by making it grounded—and more practical by making it guided.",56,158,1040,220,49,C.white,true);
  rect(s,56,430,1168,2,C.cyan);
  text(s,"Next: validate the setup experience with real administrators, then harden OCR, background indexing, security, and scale.",56,470,1040,92,24,C.white,false);
  text(s,"Questions & stakeholder feedback",56,626,760,26,18,C.cyan,true);
  note(s,"The project’s central conclusion is that local AI for schools becomes more useful when answers are grounded and more practical when deployment is guided. Archivist demonstrates that technical path with a working MVP and measurable retrieval gains. The next decision is whether to proceed to stakeholder usability testing, followed by hardening work such as OCR, background indexing, security controls, and scalable vector storage. I’d welcome questions and feedback on the workflow and the next evaluation phase.");
}

await fs.mkdir(`${BUILD}/rendered`,{recursive:true});
for (const [i,s] of p.slides.items.entries()) {
  const png=await p.export({slide:s,format:"png",scale:1});
  await fs.writeFile(`${BUILD}/rendered/slide-${i+1}.png`,new Uint8Array(await png.arrayBuffer()));
  const layout=await s.export({format:"layout"});
  await fs.writeFile(`${BUILD}/rendered/slide-${i+1}.layout.json`,await layout.text());
}
const montage=await p.export({format:"webp",montage:true,scale:1});
await fs.writeFile(`${BUILD}/montage.webp`,new Uint8Array(await montage.arrayBuffer()));
const pptx=await PresentationFile.exportPptx(p); await pptx.save(OUT);
console.log(OUT);
