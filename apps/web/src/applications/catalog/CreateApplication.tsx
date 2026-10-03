import { useEffect, useState } from 'react';
import { Modal } from '../../Modal';

export function CreateApplication({ close, onDirty }: { close: () => void; onDirty: (dirty: boolean) => void }) {
 const [name, setName] = useState('');
 const [confirm, setConfirm] = useState(false);
 useEffect(() => { onDirty(name.length > 0); return () => onDirty(false); }, [name, onDirty]);
 const requestClose = () => name.length ? setConfirm(true) : close();
 return <><Modal title="新建应用" onClose={requestClose}>
  <form className="app-create-form" onSubmit={e => e.preventDefault()}>
   <label>应用名称<input autoFocus name="application-name" value={name} onChange={e => setName(e.target.value)} /></label>
   <div className="dialog-actions"><button type="button" className="admin-button" onClick={requestClose}>取消</button><button type="submit" className="admin-button primary">创建应用</button></div>
  </form>
 </Modal>{confirm && <Modal title="有未保存的修改" onClose={() => setConfirm(false)}><p>关闭将丢失未保存的应用名称。</p><div className="dialog-actions"><button className="admin-button" onClick={() => setConfirm(false)}>继续编辑</button><button className="admin-button primary" onClick={close}>放弃修改</button></div></Modal>}</>;
}

