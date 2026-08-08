#!/usr/bin/env node
// 验证蒙皮 glTF 导出链路：从 API 拉 skin3d 数据 → 重建 SkinnedMesh + AnimationClip →
// GLTFExporter 导出 .glb → 解析 GLB（magic + JSON chunk）确认含 skin + 动画。
// 与前端 SkinDemoView 的导出逻辑一致（Node 端复现，供自动化验证 / 批处理导出）。
//
// 用法: node scripts/verify_gltf_export.mjs [action] [out.glb]

// GLTFExporter 二进制输出依赖浏览器 FileReader → Node 环境 polyfill
if (typeof globalThis.FileReader === 'undefined') {
  globalThis.FileReader = class {
    readAsArrayBuffer(blob) {
      const fr = this
      const p = typeof blob === 'string' ? Promise.resolve(new Uint8Array(Buffer.from(blob, 'base64')).buffer)
        : (blob && blob.arrayBuffer ? blob.arrayBuffer() : Promise.resolve(blob))
      p.then((buf) => { fr.result = buf; fr.onloadend && fr.onloadend({ target: fr }) })
        .catch((e) => fr.onerror && fr.onerror(e))
    }
  }
}

import * as THREE from '../creatureforge/web/node_modules/three/build/three.module.js'
import { GLTFExporter } from '../creatureforge/web/node_modules/three/examples/jsm/exporters/GLTFExporter.js'
import { writeFileSync } from 'node:fs'

const action = process.argv[2] || 'walk3d'
const outFile = process.argv[3] || `/tmp/cf_${action}.glb`

const flipY = (arr) => {
  const a = new Float32Array(arr.length)
  for (let i = 0; i < arr.length; i += 3) { a[i] = arr[i]; a[i + 1] = -arr[i + 1]; a[i + 2] = arr[i + 2] }
  return a
}

async function main() {
  const d = await (await fetch(`http://127.0.0.1:8765/api/skin3d/${action}?species=human`)).json()
  if (!d.ok) throw new Error('API 返回失败: ' + JSON.stringify(d))
  const { fkTree, bind, bn, m, nv, frames, weights, trs, fps } = {
    fkTree: d.fk_tree || {}, bind: d.bindJoints || {}, bn: d.boneNames || [],
    m: d.mesh || {}, nv: d.mesh?.vertex_count || 0, frames: d.frames || [],
    weights: d.weights || [], trs: d.trs || [], fps: d.fps || 6,
  }
  // 骨骼层级（与前端 buildSkinned 一致）
  const indexOf = {}; bn.forEach((n, i) => { indexOf[n] = i })
  const bones = bn.map((name) => { const b = new THREE.Bone(); b.name = name; return b })
  const byParent = {}
  for (const n of bn) { const p = fkTree[n]; if (p != null) (byParent[p] = byParent[p] || []).push(n) }
  ;(function place(j, parentIdx) {
    const bi = indexOf[j]; const p = bind[j]
    if (parentIdx != null) {
      const pp = bind[bn[parentIdx]]
      bones[bi].position.set(p[0] - pp[0], -(p[1] - pp[1]), p[2] - pp[2])
      bones[parentIdx].add(bones[bi])
    } else {
      bones[bi].position.set(p[0], -p[1], p[2])
    }
    for (const c of (byParent[j] || [])) place(c, bi)
  })(Object.keys(fkTree).find((j) => fkTree[j] == null), null)
  // 几何 + 权重
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(flipY(frames[0] || []), 3))
  if (m.uvs?.length) geo.setAttribute('uv', new THREE.BufferAttribute(new Float32Array(m.uvs), 2))
  if (m.normals?.length) geo.setAttribute('normal', new THREE.BufferAttribute(flipY(m.normals), 3))
  if (m.indices?.length) geo.setIndex(m.indices)
  const si = new Float32Array(nv * 4), sw = new Float32Array(nv * 4)
  weights.forEach((w, vi) => {
    for (let k = 0; k < (w.indices || []).length && k < 4; k++) {
      si[vi * 4 + k] = w.indices[k]; sw[vi * 4 + k] = w.weights[k]
    }
  })
  geo.setAttribute('skinIndex', new THREE.BufferAttribute(si, 4))
  geo.setAttribute('skinWeight', new THREE.BufferAttribute(sw, 4))
  const mat = new THREE.MeshStandardMaterial({ color: 0xc9a58c, roughness: 0.6, side: THREE.DoubleSide })
  const skinned = new THREE.SkinnedMesh(geo, mat)
  skinned.name = 'creatureforge'  // 动画 track 前缀
  const skeleton = new THREE.Skeleton(bones)
  skinned.add(bones[0])
  skinned.bind(skeleton)
  skeleton.update(); skinned.updateMatrixWorld(true)
  // 动画（与前端 buildClip 一致）
  if (trs.length) {
    const n = trs.length
    const times = []; for (let i = 0; i < n; i++) times.push(i / fps)
    const tracks = []
    const euler = new THREE.Euler(), quat = new THREE.Quaternion()
    const prefix = 'creatureforge'
    for (const name of bn) {
      const vals = new Float32Array(n * 4)
      trs.forEach((fr, i) => {
        const r = (fr.rot && fr.rot[name]) || [0, 0, 0]
        euler.set(r[0], r[1], r[2], 'XYZ'); quat.setFromEuler(euler)
        vals[i * 4] = quat.x; vals[i * 4 + 1] = quat.y; vals[i * 4 + 2] = quat.z; vals[i * 4 + 3] = quat.w
      })
      tracks.push(new THREE.QuaternionKeyframeTrack(`${prefix}.bones[${name}].quaternion`, times, vals))
    }
    const rootName = bn[0], bindRoot = bind[rootName]
    if (bindRoot && trs[0]?.root) {
      const vals = new Float32Array(n * 3)
      trs.forEach((fr, i) => {
        vals[i * 3] = bindRoot[0] + fr.root[0]
        vals[i * 3 + 1] = -bindRoot[1] + fr.root[1]
        vals[i * 3 + 2] = bindRoot[2] + fr.root[2]
      })
      tracks.push(new THREE.VectorKeyframeTrack(`${prefix}.bones[${rootName}].position`, times, vals))
    }
    skinned.animations = [new THREE.AnimationClip(action, n / fps, tracks)]
  }
  // 导出 GLB（animations 经 options 传入）
  const exporter = new GLTFExporter()
  const anims = skinned.animations || []
  const result = await new Promise((res, rej) =>
    exporter.parse(skinned, res, rej, { binary: true, animations: anims }))
  const buf = result instanceof ArrayBuffer ? result : result.buffer
  writeFileSync(outFile, Buffer.from(buf))
  // 解析 GLB：magic + JSON chunk
  const magic = new TextDecoder().decode(new Uint8Array(buf, 0, 4))
  const dv = new DataView(buf)
  const jsonLen = dv.getUint32(12, true)
  const json = JSON.parse(new TextDecoder().decode(new Uint8Array(buf, 20, jsonLen)))
  console.log(`GLB magic: ${magic}  (${(buf.byteLength / 1024).toFixed(0)}KB)`)
  console.log(`animations: ${json.animations?.length || 0}  skins: ${json.skins?.length || 0}  meshes: ${json.meshes?.length || 0}  nodes: ${json.nodes?.length || 0}`)
  const ok = magic === 'glTF' && (json.animations?.length || 0) > 0 && (json.skins?.length || 0) > 0
  console.log(ok ? '✓ 导出含皮肤 + 动画' : '✗ 缺少 skin/动画')
  if (!ok) process.exit(1)
}

main().catch((e) => { console.error(e); process.exit(1) })
