create or replace package reg is

  -- Author  : ГУСЕЙНОВ_Ш
  -- Created : 02.09.2024 10:41:01
  -- Purpose : 
  
  -- Public type declarations
  procedure add_head(iname in varchar2);
  procedure del_head(iname in varchar2);  
  procedure modify_head(iname in varchar2);  
  function add_reg(itime_out in varchar2, itime_in in varchar2, iemployee in varchar2, 
                   ipost in varchar2, idep_name in varchar2, icause in varchar2) return varchar2;
  function add_secure_reg(itime_out in varchar2, itime_in in varchar2, iemployee in varchar2, 
                    ipost in varchar2, idep_name in varchar2, icause in varchar2, iboss in varchar2) return varchar2;
  procedure del_time_off(id_reg in pls_integer);
  procedure approve_time_off(id_reg in pls_integer, iboss in varchar2);
  procedure refuse_time_off(id_reg in pls_integer, iboss in varchar2);
  procedure fact_time_off(id_reg in pls_integer);
  procedure new_message(iauthor in varchar2, idep_name in varchar2, imessage in varchar2, 
            iphoto_url in varchar2, iphoto_fio in varchar2, iphoto_post in varchar2);
  procedure new_message_2(iauthor in varchar2, idep_name in varchar2, imessage in varchar2, 
    iphoto_url in varchar2, iphoto_fio in varchar2, iphoto_post in varchar2);
 
  procedure use_file_statistic(i_username in varchar2, i_dep_name in varchar2, i_file_name in varchar2, i_file_path in varchar2);
  PROCEDURE del_message(p_id_mess IN NUMBER, p_author  IN VARCHAR2, p_ip      IN VARCHAR2 DEFAULT NULL);
end reg;
/
create or replace package body reg is


  procedure log(iproc in varchar2, imsg in varchar2)
  is
  pragma autonomous_transaction;
  begin
    insert into log(package, proc, msg) values('registr', iproc, imsg);
    commit;
  end log;
    
  procedure add_head(iname in varchar2)
  is
    v_id pls_integer default 0;
  begin
    begin
        select id_head+1 into v_id
        from (
          select id_head, 
                 lead(id_head,1) over(order by id_head) next
          from heads h
          order by id_head 
          ) 
        where next - id_head > 1
        and rownum=1;
    exception when no_data_found then
        select coalesce(max(id_head),0)+1 into v_id from heads;
    end;
    
    insert into heads(id_head, name) values(v_id, iname);
    commit;
  end add_head;
  
  procedure del_head(iname in varchar2)
  is
  begin
    delete from heads h where h.name=iname;
    commit;
    if sql%rowcount=0 then 
      log('modify_head', '1. Head not found: '||iname);
    end if;
    exception when no_data_found then 
      log('modify_head', 'Head not found: '||iname);
  end del_head;  
  
  procedure modify_head(iname in varchar2)
  is
    v_id_head pls_integer default 0;
  begin
    select id_head into v_id_head from heads h where h.name=iname;
    update heads h 
    set    h.name=iname
    where  h.id_head=v_id_head;
    exception when no_data_found then 
      log('modify_head', 'Head not found: '||iname);
  end modify_head;  

  function add_reg(itime_out in varchar2, itime_in in varchar2, iemployee in varchar2, 
                    ipost in varchar2, idep_name in varchar2, icause in varchar2)
                    return varchar2
  is
    v_time_out date;
    v_time_in  date;
    msg varchar2(256);
  begin
    v_time_out:=to_date(itime_out,'YYYY-MM-DD HH24:MI');
    v_time_in:=to_date(itime_in,'YYYY-MM-DD HH24:MI');
    if trunc(v_time_out,'DD')<trunc(sysdate,'DD') then
--        msg:='Регистрируемая дата выхода '||itime_out||' истекла';
       msg:='Регистрируемая дата выхода '||to_char(v_time_out,'dd.mm.yyyy')||' истекла';
    end if;
    
    if v_time_out>v_time_in then
       msg:=msg || case when msg is null then '' else ', ' end ||'Время ухода позже времени прихода';
    end if;
    
    if icause is null then
       msg:=msg || case when msg is null then '' else ', ' end ||'Не указана причина выхода';
    end if;
    
    if msg is not null then
       log('add_reg', msg);
       return msg;
    end if;
      
    insert into register(time_out, time_in, employee, post, dep_name, cause, status)
    values(to_date(itime_out,'YYYY-MM-DD HH24:MI'), to_date(itime_in,'YYYY-MM-DD HH24:MI'), iemployee, ipost, idep_name, icause, 0);
    commit;
    return 'Success';

  end add_reg;

  function add_secure_reg(itime_out in varchar2, itime_in in varchar2, iemployee in varchar2, 
                    ipost in varchar2, idep_name in varchar2, icause in varchar2, iboss in varchar2)
                    return varchar2
  is
    v_time_out date;
    v_time_in  date;
    msg varchar2(256);
  begin
    v_time_out:=to_date(itime_out,'YYYY-MM-DD HH24:MI');
    v_time_in:=to_date(itime_in,'YYYY-MM-DD HH24:MI');
    
    if v_time_out>v_time_in then
       msg:=msg || case when msg is null then '' else ', ' end ||'Время ухода больше времени прихода';
    end if;
    
    if icause is null then
       msg:=msg || case when msg is null then '' else ', ' end ||'Не указана причина выхода';
    end if;

    if iboss is null then
       msg:=msg || case when msg is null then '' else ', ' end ||'Не указан сотрудник СБ, принимающий решение';
    end if;
    
    if msg is not null then
       log('add_secure_reg', msg);
       return msg;
    end if;
      
    insert into register(time_out, time_in, employee, post, dep_name, cause, status, head)
    values(to_date(itime_out,'YYYY-MM-DD HH24:MI'), to_date(itime_in,'YYYY-MM-DD HH24:MI'), iemployee, ipost, idep_name, icause, 3, iboss);
    commit;
    return 'Success';

  end add_secure_reg;

  procedure del_time_off(id_reg in pls_integer)
  is
  begin
    delete from register r where r.id=id_reg;
    commit;
  end del_time_off;

  procedure approve_time_off(id_reg in pls_integer, iboss in varchar2)
  is
  begin
    update register r 
    set    r.head=iboss,
           r.status=1
    where r.id=id_reg;
   
    commit;
    log('approve_time_off', 'id_reg: '||id_reg||', head: '||iboss);
  end approve_time_off;

  procedure refuse_time_off(id_reg in pls_integer, iboss in varchar2)
  is
  begin
    update register r 
    set    r.head=iboss,
           r.status=2
    where r.id=id_reg;
   
    commit;
    log('refuse_time_off', 'id_reg: '||id_reg||', head: '||iboss);
  end refuse_time_off;

  procedure fact_time_off(id_reg in pls_integer)
  is
  begin
    
    update register r 
    set    r.time_fact=sysdate
    where r.id=id_reg;
   
    commit;
    log('fact_time_off', 'id_reg: '||id_reg);
  end fact_time_off;


  procedure new_message(iauthor in varchar2, idep_name in varchar2, imessage in varchar2, 
            iphoto_url in varchar2, iphoto_fio in varchar2, iphoto_post in varchar2)
  is
  begin
    
    insert into messages( id_mess, author, dep_name, message, photo_url, photo_fio, photo_post)
    values (seq_id_message.nextval, iauthor, idep_name, imessage, iphoto_url, iphoto_fio, iphoto_post);
   
    commit;
    log('new_message', 'authhor: '||iauthor||', dep_name: '||idep_name||', message: '||imessage);
  end new_message;

  procedure new_message_2(iauthor in varchar2, idep_name in varchar2, imessage in varchar2, 
    iphoto_url in varchar2, iphoto_fio in varchar2, iphoto_post in varchar2)
  is
  begin
    
    insert into messages_2( id_mess, author, dep_name, message, photo_url, photo_fio, photo_post)
    values (seq_id_message.nextval, iauthor, idep_name, imessage, iphoto_url, iphoto_fio, iphoto_post);
   
    commit;
    log('new_message', 'authhor: '||iauthor||', dep_name: '||idep_name||', message: '||imessage);
  end new_message_2;
  
  procedure use_file_statistic(i_username in varchar2, i_dep_name in varchar2, i_file_name in varchar2, i_file_path in varchar2)
  is
  begin
    insert into use_doc( date_op, user_name, dep_name, file_name, file_path)
    values ( sysdate, i_username, i_dep_name, i_file_name, i_file_path);
   
    commit;
    log('use_file_statistic', 'username: '||i_username||', dep_name: '||i_dep_name||', file_name: '||i_file_name);
  end use_file_statistic;
  
-- =============================== ТЕЛО ================================
-- Добавить в PACKAGE BODY reg:

  PROCEDURE del_message(
    p_id_mess IN NUMBER,
    p_author  IN VARCHAR2,
    p_ip      IN VARCHAR2 DEFAULT NULL
  ) IS
    v_deleted PLS_INTEGER;
  BEGIN
    log('del_message', 'id_mess=' || p_id_mess || ', author=' || p_author || ', ip=' || NVL(p_ip, '-'));

    -- Ключевое условие — AND author = p_author. Без него процедура превращается
    -- в "удали любую новость по номеру".
    DELETE FROM messages
     WHERE id_mess = p_id_mess
       AND TRIM(author) = TRIM(p_author);

    v_deleted := SQL%ROWCOUNT;

    IF v_deleted = 0 THEN
      -- Ноль строк — это либо чужая новость, либо уже удалённая. Различать их
      -- в ответе не нужно (и не стоит), но в логе пакета след должен остаться:
      -- по нему видно попытки удалить не своё, если они пойдут потоком.
      log('del_message', 'Не удалено (нет новости или не автор): id_mess=' ||
                          p_id_mess || ', author=' || p_author);
      -- Ошибку НЕ поднимаем: Go уже сверил автора и показал человеку понятный
      -- текст, а гонка "двое удаляют одну новость" — не аварийная ситуация.
      null;
    END IF;

    COMMIT;

  EXCEPTION
    WHEN OTHERS THEN
      log('del_message', 'ERROR' || SQLERRM);
      RAISE;
  END del_message;
  
begin
  -- Initialization
  null;
end reg;
/
